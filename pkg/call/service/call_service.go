package call_service

import (
	"context"
	"errors"
	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	call_stream "github.com/evolution-foundation/evolution-go/pkg/call/stream"
	"github.com/evolution-foundation/evolution-go/pkg/safemap"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"strings"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gomessguii/logger"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type CallService interface {
	RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error
	// ActiveCalls reports the call engine of the instance and the calls it is following.
	ActiveCalls(instance *instance_model.Instance) ActiveCallsResult
	// GetCall describes one call the instance is following.
	GetCall(instance *instance_model.Instance, callID string) (call_engine.Info, error)
	AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (call_engine.Info, error)
	HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error
	// IssueStreamTicket creates the one-time ticket that opens the audio stream of a call.
	IssueStreamTicket(instance *instance_model.Instance, callID string, video bool) (StreamTicket, error)
	// VideoCall does one video action (start, accept, stop, enable, disable, orientation).
	VideoCall(instance *instance_model.Instance, data *VideoCallStruct) (call_engine.Info, error)
	// DialCall places an outgoing call.
	DialCall(ctx context.Context, data *DialCallStruct, instance *instance_model.Instance) (DialResult, error)
}

// ErrInvalidNumber: the number is not one a call can be placed to (a group, a
// broadcast list, something that is not a number).
var ErrInvalidNumber = errors.New("number must be a WhatsApp user (a phone number or a @lid), not a group or list")

type DialCallStruct struct {
	// Number is a phone number or a user JID. The number-validation middleware turns a
	// phone number into a JID before this is read.
	Number string `json:"number"`
	// Stream also returns the ticket that opens the stream, so it can be connected
	// before the callee picks up and says hello.
	Stream bool `json:"stream"`
	// Video places a video call. With Stream, the stream carries the video too.
	Video bool `json:"video"`
}

// DialResult is the answer of POST /call/dial: the call, and its stream ticket when one
// was asked for.
type DialResult struct {
	call_engine.Info
	StreamTicket *StreamTicket `json:"streamTicket,omitempty"`
}

// ErrCallsUnavailable: the running client of the instance has no working call engine.
var ErrCallsUnavailable = errors.New("calls are not active for this instance: turn on callsEnabled and reconnect it (instances that use a proxy cannot make calls)")

type AnswerCallStruct struct {
	CallID string `json:"callId"`
}

type HangupCallStruct struct {
	CallID string `json:"callId"`
}

type StreamTicketStruct struct {
	CallID string `json:"callId"`
	// Video makes the stream carry the call's video besides its audio.
	Video bool `json:"video"`
}

// VideoCallStruct is the body of POST /call/video.
type VideoCallStruct struct {
	CallID string `json:"callId"`
	// Action is one of: start (ask the peer to turn the call into a video call),
	// accept (accept the peer's request), stop, enable, disable, orientation.
	Action string `json:"action"`
	// Orientation is how the camera is rotated, in clockwise quarter turns (0..3); only
	// read by the "orientation" action.
	Orientation int `json:"orientation"`
}

// StreamTicket is the answer of POST /call/stream-ticket: open a WebSocket to Path
// (on this server) before ExpiresInSeconds pass.
type StreamTicket struct {
	Ticket           string `json:"ticket"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	Path             string `json:"path"`
}

// ActiveCallsResult is the answer of GET /call/active.
type ActiveCallsResult struct {
	// Enabled is false when the running client has no call engine (calls are off for
	// the instance, or it has not reconnected since they were turned on).
	Enabled bool               `json:"enabled"`
	State   call_engine.State  `json:"state,omitempty"`
	Error   string             `json:"error,omitempty"`
	Calls   []call_engine.Info `json:"calls"`
}

type callService struct {
	clientPointer    *safemap.Map[*whatsmeow.Client]
	whatsmeowService whatsmeow_service.WhatsmeowService
	tickets          *call_stream.Tickets
	loggerWrapper    *logger_wrapper.LoggerManager
}

type RejectCallStruct struct {
	CallCreator types.JID `json:"callCreator"`
	CallID      string    `json:"callId"`
}

func (c *callService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := c.clientPointer.Get(instanceId)
	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := c.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting for the connection...", instanceId)
		client = utils.WaitForClient(func() *whatsmeow.Client { return c.clientPointer.Get(instanceId) }, utils.InstanceStartTimeout)
		c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			c.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		c.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	c.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

// engine returns the call engine of the instance, or ErrCallsUnavailable when the
// running client has none that works.
func (c *callService) engine(instance *instance_model.Instance) (*call_engine.Manager, error) {
	engine := c.whatsmeowService.CallEngine()
	if st, ok := engine.Status(instance.Id); !ok || st.State != call_engine.StateActive {
		return nil, ErrCallsUnavailable
	}
	return engine, nil
}

func (c *callService) GetCall(instance *instance_model.Instance, callID string) (call_engine.Info, error) {
	engine, err := c.engine(instance)
	if err != nil {
		return call_engine.Info{}, err
	}
	t, ok := engine.Get(instance.Id, callID)
	if !ok {
		return call_engine.Info{}, call_engine.ErrCallNotFound
	}
	return t.Info(), nil
}

func (c *callService) AnswerCall(data *AnswerCallStruct, instance *instance_model.Instance) (call_engine.Info, error) {
	engine, err := c.engine(instance)
	if err != nil {
		return call_engine.Info{}, err
	}
	t, err := engine.Answer(instance.Id, data.CallID)
	if err != nil {
		logger.LogError("[%s] error answering call %s: %v", instance.Id, data.CallID, err)
		return call_engine.Info{}, err
	}
	return t.Info(), nil
}

func (c *callService) HangupCall(data *HangupCallStruct, instance *instance_model.Instance) error {
	engine, err := c.engine(instance)
	if err != nil {
		return err
	}
	_, err = engine.Hangup(instance.Id, data.CallID)
	if err != nil && !errors.Is(err, call_engine.ErrCallNotFound) {
		// The call is over here; only telling the peer failed.
		logger.LogError("[%s] error hanging up call %s: %v", instance.Id, data.CallID, err)
	}
	return err
}

// dialTarget checks that number is a user a call can go to and returns it as the
// library wants it. The call goes out in a raw node, which the server silently drops
// when the JID has the "+" that CreateJID puts on phone numbers, hence CanonicalJID.
func dialTarget(number string) (string, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return "", ErrInvalidNumber
	}
	if !strings.Contains(number, "@") {
		jid, err := utils.CreateJID(number)
		if err != nil {
			return "", ErrInvalidNumber
		}
		number = jid
	}
	jid, err := types.ParseJID(number)
	if err != nil || jid.User == "" || (jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
		return "", ErrInvalidNumber
	}
	return utils.CanonicalJID(jid.ToNonAD()).String(), nil
}

func (c *callService) DialCall(ctx context.Context, data *DialCallStruct, instance *instance_model.Instance) (DialResult, error) {
	engine, err := c.engine(instance)
	if err != nil {
		return DialResult{}, err
	}
	target, err := dialTarget(data.Number)
	if err != nil {
		return DialResult{}, err
	}

	t, err := engine.Dial(ctx, instance.Id, target, call_engine.DialOptions{Video: data.Video})
	if err != nil {
		logger.LogError("[%s] error dialing call: %v", instance.Id, err)
		return DialResult{}, err
	}
	result := DialResult{Info: t.Info()}

	if data.Stream {
		ticket, err := c.IssueStreamTicket(instance, result.CallID, data.Video)
		if err != nil {
			// The caller asked for a stream it cannot have: do not leave the call ringing.
			_, _ = engine.Hangup(instance.Id, result.CallID)
			return DialResult{}, err
		}
		result.StreamTicket = &ticket
	}
	return result, nil
}

func (c *callService) VideoCall(instance *instance_model.Instance, data *VideoCallStruct) (call_engine.Info, error) {
	engine, err := c.engine(instance)
	if err != nil {
		return call_engine.Info{}, err
	}
	t, err := engine.Video(instance.Id, data.CallID, call_engine.VideoAction(data.Action), data.Orientation)
	if err != nil {
		logger.LogError("[%s] error on video %q of call %s: %v", instance.Id, data.Action, data.CallID, err)
		return call_engine.Info{}, err
	}
	return t.Info(), nil
}

func (c *callService) IssueStreamTicket(instance *instance_model.Instance, callID string, video bool) (StreamTicket, error) {
	engine, err := c.engine(instance)
	if err != nil {
		return StreamTicket{}, err
	}
	if _, ok := engine.Get(instance.Id, callID); !ok {
		return StreamTicket{}, call_engine.ErrCallNotFound
	}
	token, ttl, err := c.tickets.Issue(instance.Id, callID, video)
	if err != nil {
		return StreamTicket{}, err
	}
	return StreamTicket{
		Ticket:           token,
		ExpiresInSeconds: int(ttl.Seconds()),
		Path:             "/call/stream/" + callID + "?ticket=" + token,
	}, nil
}

func (c *callService) ActiveCalls(instance *instance_model.Instance) ActiveCallsResult {
	engine := c.whatsmeowService.CallEngine()
	result := ActiveCallsResult{Calls: engine.List(instance.Id)}
	if st, ok := engine.Status(instance.Id); ok {
		result.Enabled = st.State == call_engine.StateActive
		result.State = st.State
		result.Error = st.Error
	}
	return result
}

func (c *callService) RejectCall(data *RejectCallStruct, instance *instance_model.Instance) error {
	// A call the engine follows (the instance preaccepted it) has to be rejected through
	// the engine, or it keeps the call in its own state.
	if tracked, err := c.whatsmeowService.CallEngine().Reject(instance.Id, data.CallID); tracked {
		if err != nil {
			logger.LogError("[%s] error reject call: %v", instance.Id, err)
		}
		return err
	}

	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	err = client.RejectCall(context.Background(), data.CallCreator, data.CallID)
	if err != nil {
		logger.LogError("[%s] error reject call: %v", instance.Id, err)
		return err
	}

	return nil
}

func NewCallService(
	clientPointer *safemap.Map[*whatsmeow.Client],
	whatsmeowService whatsmeow_service.WhatsmeowService,
	tickets *call_stream.Tickets,
	loggerWrapper *logger_wrapper.LoggerManager,
) CallService {
	return &callService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		tickets:          tickets,
		loggerWrapper:    loggerWrapper,
	}
}

package call_service

import (
	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	"github.com/evolution-foundation/evolution-go/pkg/safemap"
	"context"
	"errors"

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
}

// ActiveCallsResult is the answer of GET /call/active.
type ActiveCallsResult struct {
	// Enabled is false when the running client has no call engine (calls are off for
	// the instance, or it has not reconnected since they were turned on).
	Enabled bool              `json:"enabled"`
	State   call_engine.State `json:"state,omitempty"`
	Error   string            `json:"error,omitempty"`
	Calls   []call_engine.Info `json:"calls"`
}

type callService struct {
	clientPointer    *safemap.Map[*whatsmeow.Client]
	whatsmeowService whatsmeow_service.WhatsmeowService
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
	loggerWrapper *logger_wrapper.LoggerManager,
) CallService {
	return &callService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}

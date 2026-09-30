package call_handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	call_stream "github.com/evolution-foundation/evolution-go/pkg/call/stream"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type CallHandler interface {
	RejectCall(ctx *gin.Context)
	ActiveCalls(ctx *gin.Context)
	GetCall(ctx *gin.Context)
	AnswerCall(ctx *gin.Context)
	HangupCall(ctx *gin.Context)
	StreamTicket(ctx *gin.Context)
	DialCall(ctx *gin.Context)
}

type callHandler struct {
	callService call_service.CallService
}

// Reject call
// @Summary Reject call
// @Description Reject call
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.RejectCallStruct true "Call data"
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/reject [post]
func (g *callHandler) RejectCall(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *call_service.RejectCallStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = g.callService.RejectCall(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Active calls
// @Summary Active calls
// @Description The call engine of the instance and the calls it is following (incoming and outgoing, until they end). "enabled" is false when the running client has no call engine.
// @Tags Call
// @Produce json
// @Success 200 {object} call_service.ActiveCallsResult
// @Failure 500 {object} gin.H "Internal server error"
// @Router /call/active [get]
func (g *callHandler) ActiveCalls(ctx *gin.Context) {
	instance, ok := ctx.MustGet("instance").(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	ctx.JSON(http.StatusOK, g.callService.ActiveCalls(instance))
}

// callFailure answers a failed call action with the status that says what went wrong.
func callFailure(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, call_engine.ErrCallNotFound):
		status = http.StatusNotFound
	case errors.Is(err, call_engine.ErrWrongState), errors.Is(err, call_service.ErrCallsUnavailable):
		status = http.StatusConflict
	case errors.Is(err, call_stream.ErrTooManyTickets), errors.Is(err, call_engine.ErrTooManyCalls), errors.Is(err, call_engine.ErrDialRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, call_service.ErrInvalidNumber):
		status = http.StatusBadRequest
	case errors.Is(err, call_engine.ErrDialFailed):
		status = http.StatusBadGateway
	}
	ctx.JSON(status, gin.H{"error": err.Error()})
}

func instanceOf(ctx *gin.Context) (*instance_model.Instance, bool) {
	instance, ok := ctx.MustGet("instance").(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
	}
	return instance, ok
}

// Get call
// @Summary Get a call
// @Description One call the instance is following: its phase, direction, whether it has video and how much audio its stream moved.
// @Tags Call
// @Produce json
// @Param callId path string true "Call id (from the CallOffer event)"
// @Success 200 {object} call_engine.Info
// @Failure 404 {object} gin.H "No such call"
// @Failure 409 {object} gin.H "Calls are not active for this instance"
// @Router /call/{callId} [get]
func (g *callHandler) GetCall(ctx *gin.Context) {
	instance, ok := instanceOf(ctx)
	if !ok {
		return
	}
	info, err := g.callService.GetCall(instance, ctx.Param("callId"))
	if err != nil {
		callFailure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, info)
}

// Answer call
// @Summary Answer an incoming call
// @Description Answers an incoming call that is still ringing. Its audio is then available on the stream (see /call/stream-ticket); a call answered without a stream is hung up after a short grace period.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.AnswerCallStruct true "Call data"
// @Success 200 {object} call_engine.Info
// @Failure 400 {object} gin.H "callId missing"
// @Failure 404 {object} gin.H "No such call"
// @Failure 409 {object} gin.H "The call is not ringing, or calls are not active for this instance"
// @Router /call/answer [post]
func (g *callHandler) AnswerCall(ctx *gin.Context) {
	instance, ok := instanceOf(ctx)
	if !ok {
		return
	}
	var data call_service.AnswerCallStruct
	if err := ctx.ShouldBindJSON(&data); err != nil || data.CallID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}
	info, err := g.callService.AnswerCall(&data, instance)
	if err != nil {
		callFailure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, info)
}

// Hang up call
// @Summary Hang up a call
// @Description Ends a call in any phase; an incoming call that still rings is rejected. The call is over here even when telling the peer fails (that answers 500).
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.HangupCallStruct true "Call data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "callId missing"
// @Failure 404 {object} gin.H "No such call"
// @Router /call/hangup [post]
func (g *callHandler) HangupCall(ctx *gin.Context) {
	instance, ok := instanceOf(ctx)
	if !ok {
		return
	}
	var data call_service.HangupCallStruct
	if err := ctx.ShouldBindJSON(&data); err != nil || data.CallID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}
	if err := g.callService.HangupCall(&data, instance); err != nil {
		callFailure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Stream ticket
// @Summary Ticket for the audio stream of a call
// @Description Returns a one-time ticket, valid for a few seconds, for one call. Open a WebSocket to "path" on this server with it: GET /call/stream/{callId}?ticket=... . Audio is 16 kHz mono 16-bit little-endian PCM in base64 JSON messages (see the "start" message).
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.StreamTicketStruct true "Call data"
// @Success 200 {object} call_service.StreamTicket
// @Failure 400 {object} gin.H "callId missing"
// @Failure 404 {object} gin.H "No such call"
// @Router /call/stream-ticket [post]
func (g *callHandler) StreamTicket(ctx *gin.Context) {
	instance, ok := instanceOf(ctx)
	if !ok {
		return
	}
	var data call_service.StreamTicketStruct
	if err := ctx.ShouldBindJSON(&data); err != nil || data.CallID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}
	ticket, err := g.callService.IssueStreamTicket(instance, data.CallID)
	if err != nil {
		callFailure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, ticket)
}

// dialTimeout bounds placing a call: resolving the number, fetching its devices,
// encrypting the call key for each and sending the offer.
const dialTimeout = 30 * time.Second

// Dial call
// @Summary Place a call
// @Description Places an outgoing audio call to a WhatsApp user and returns it in the "calling" phase; it rings on the other phone. With "stream": true the answer also carries a ticket for the audio stream, so it can be connected before the callee picks up. Video is not supported yet. An instance may place a limited number of calls per minute (CALL_DIAL_LIMIT) and have a limited number at once (CALL_MAX_CONCURRENT): both answer 429.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.DialCallStruct true "Who to call"
// @Success 200 {object} call_service.DialResult
// @Failure 400 {object} gin.H "Not a number a call can go to"
// @Failure 409 {object} gin.H "Calls are not active for this instance"
// @Failure 429 {object} gin.H "Too many calls"
// @Failure 502 {object} gin.H "WhatsApp or the library could not place the call"
// @Router /call/dial [post]
func (g *callHandler) DialCall(ctx *gin.Context) {
	instance, ok := instanceOf(ctx)
	if !ok {
		return
	}
	var data call_service.DialCallStruct
	if err := ctx.ShouldBindJSON(&data); err != nil || data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "number is required (one number, not a list)"})
		return
	}

	dialCtx, cancel := context.WithTimeout(ctx.Request.Context(), dialTimeout)
	defer cancel()
	result, err := g.callService.DialCall(dialCtx, &data, instance)
	if err != nil {
		callFailure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func NewCallHandler(
	callService call_service.CallService,
) CallHandler {
	return &callHandler{
		callService: callService,
	}
}

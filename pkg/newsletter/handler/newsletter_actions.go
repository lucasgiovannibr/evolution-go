package newsletter_handler

import (
	"net/http"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	newsletter_service "github.com/evolution-foundation/evolution-go/pkg/newsletter/service"
	"github.com/gin-gonic/gin"
)

// bindChannelAction runs the common part of the channel actions: the instance, the
// body, and the mapping of the outcome to a status code.
func bindChannelAction[T any](ctx *gin.Context, act func(data *T, instance *instance_model.Instance) error) {
	instance, ok := ctx.MustGet("instance").(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *T
	if err := ctx.ShouldBindBodyWithJSON(&data); err != nil || data == nil {
		msg := "invalid body"
		if err != nil {
			msg = err.Error()
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	if err := act(data, instance); err != nil {
		status := http.StatusInternalServerError
		if newsletter_service.IsNewsletterRequestError(err) {
			status = http.StatusBadRequest
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Follow a channel
// @Summary Follow a channel
// @Tags Newsletter
// @Accept json
// @Produce json
// @Param message body newsletter_service.GetNewsletterStruct true "Channel"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /newsletter/follow [post]
func (n *newsletterHandler) FollowNewsletter(ctx *gin.Context) {
	bindChannelAction(ctx, n.newsletterService.FollowNewsletter)
}

// Unfollow a channel
// @Summary Unfollow a channel
// @Tags Newsletter
// @Accept json
// @Produce json
// @Param message body newsletter_service.GetNewsletterStruct true "Channel"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /newsletter/unfollow [post]
func (n *newsletterHandler) UnfollowNewsletter(ctx *gin.Context) {
	bindChannelAction(ctx, n.newsletterService.UnfollowNewsletter)
}

// Mute or unmute a channel
// @Summary Mute or unmute a channel
// @Tags Newsletter
// @Accept json
// @Produce json
// @Param message body newsletter_service.NewsletterMuteStruct true "Channel and mute flag"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /newsletter/mute [post]
func (n *newsletterHandler) MuteNewsletter(ctx *gin.Context) {
	bindChannelAction(ctx, n.newsletterService.MuteNewsletter)
}

// Mark channel messages as viewed
// @Summary Mark channel messages as viewed
// @Description Counts a view on each message (serverIds). It does not mark the channel as read on the other devices.
// @Tags Newsletter
// @Accept json
// @Produce json
// @Param message body newsletter_service.NewsletterMarkViewedStruct true "Channel and message server ids"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /newsletter/markviewed [post]
func (n *newsletterHandler) MarkNewsletterViewed(ctx *gin.Context) {
	bindChannelAction(ctx, n.newsletterService.MarkNewsletterViewed)
}

// React to a channel message
// @Summary React to a channel message
// @Description Send an emoji reaction to a channel message; an empty reaction removes the one sent earlier.
// @Tags Newsletter
// @Accept json
// @Produce json
// @Param message body newsletter_service.NewsletterReactStruct true "Channel, message server id and reaction"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /newsletter/react [post]
func (n *newsletterHandler) ReactNewsletter(ctx *gin.Context) {
	bindChannelAction(ctx, n.newsletterService.ReactNewsletter)
}

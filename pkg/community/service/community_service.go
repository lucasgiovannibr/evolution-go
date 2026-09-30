package community_service

import (
	"github.com/evolution-foundation/evolution-go/pkg/safemap"
	"context"
	"errors"

	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/evolution-foundation/evolution-go/pkg/utils"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type CommunityService interface {
	CreateCommunity(data *CreateCommunityStruct, instance *instance_model.Instance) (*types.GroupInfo, error)
	CommunityAdd(data *AddParticipantStruct, instance *instance_model.Instance) (gin.H, error)
	CommunityRemove(data *AddParticipantStruct, instance *instance_model.Instance) (gin.H, error)
}

type communityService struct {
	clientPointer    *safemap.Map[*whatsmeow.Client]
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
}

type CreateCommunityStruct struct {
	CommunityName string `json:"communityName"`
}

type AddParticipantStruct struct {
	CommunityJID string   `json:"communityJid"`
	GroupJID     []string `json:"groupJid"`
}

func (c *communityService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
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

func (c *communityService) CreateCommunity(data *CreateCommunityStruct, instance *instance_model.Instance) (*types.GroupInfo, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	resp, err := client.CreateGroup(context.Background(), whatsmeow.ReqCreateGroup{
		Name: data.CommunityName,
		GroupParent: types.GroupParent{
			IsParent: true,
		},
	})
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error create community: %v", instance.Id, err)
		return nil, err
	}

	return resp, nil
}

func (c *communityService) CommunityAdd(data *AddParticipantStruct, instance *instance_model.Instance) (gin.H, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	communityJID, ok := utils.ParseJID(data.CommunityJID)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error parse community jid", instance.Id)
		return nil, errors.New("error parse community jid")
	}

	successList, failedList := applyToGroups(data.GroupJID, func(groupJID types.JID) error {
		err := client.LinkGroup(context.Background(), communityJID, groupJID)
		if err != nil {
			c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error link group: %v", instance.Id, err)
		}
		return err
	})

	return gin.H{
		"success": successList,
		"failed":  failedList,
	}, nil
}

func (c *communityService) CommunityRemove(data *AddParticipantStruct, instance *instance_model.Instance) (gin.H, error) {
	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	communityJID, ok := utils.ParseJID(data.CommunityJID)
	if !ok {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error parse community jid", instance.Id)
		return nil, errors.New("error parse community jid")
	}

	successList, failedList := applyToGroups(data.GroupJID, func(groupJID types.JID) error {
		err := client.UnlinkGroup(context.Background(), communityJID, groupJID)
		if err != nil {
			c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unlink group: %v", instance.Id, err)
		}
		return err
	})

	return gin.H{
		"success": successList,
		"failed":  failedList,
	}, nil
}

func NewCommunityService(
	clientPointer *safemap.Map[*whatsmeow.Client],
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) CommunityService {
	return &communityService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}

// applyToGroups runs op on each group and reports which ones worked and which did not.
// A group that cannot be parsed is reported as failed instead of being sent as a
// zero JID. (The lists used to be built wrongly: every group, failed or not, ended up
// in "success", made of the failed list plus the current group.)
func applyToGroups(groups []string, op func(groupJID types.JID) error) (success, failed []string) {
	for _, g := range groups {
		groupJID, ok := utils.ParseJID(g)
		if !ok {
			failed = append(failed, g)
			continue
		}
		if err := op(groupJID); err != nil {
			failed = append(failed, groupJID.String())
			continue
		}
		success = append(success, groupJID.String())
	}
	return success, failed
}

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

const (
	commandTrigger = "silencer"

	commandHelp = "###### User Silencer\n" +
		"- `/silencer` - Show a list of currently silenced users.\n" +
		"- `/silencer @user` - Toggle silencing user by name.\n" +
		"- `/silencer clear` - Clear the list of silenced users.\n" +
		"- `/silencer help` - Show this help text."
)

func (p *Plugin) OnActivate() error {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		return err
	}

	if err := p.API.RegisterCommand(&model.Command{

		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Toggles silencing of users.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTrigger)
	}

	return nil
}

func (p *Plugin) notifyListChanged(blockList []string)  {
	p.API.PublishWebSocketEvent("silencer_list_changed", map[string]interface{}{
		"list": blockList,
	}, &model.WebsocketBroadcast{})
}

func (p *Plugin) writeSilencerList(senderID string, blockList []string) error {
	kvKey := fmt.Sprintf("%v-block-list", senderID)
	blockListBytes, _ := json.Marshal(blockList)

	if err := p.API.KVSet(kvKey, blockListBytes); err != nil {
		p.API.LogError("Unable to save to kv store", "error", err.Error())
		return errors.Wrapf(err, "Unable to save")
	}

	p.notifyListChanged(blockList)
	return nil
}

func (p *Plugin) readSilencerList(senderID string) ([]string, error) {
	kvKey := fmt.Sprintf("%v-block-list", senderID)
	blockListBytes, _ := p.API.KVGet(kvKey)
	if blockListBytes == nil {
		blockListBytes, _ = json.Marshal(make([]string, 0))
		p.API.LogError("read kv failed, blockListBytes is nil")
	} else {
		p.API.LogError("read kv", "blockListBytes", blockListBytes)
	}
	var blockList []string
	if err := json.Unmarshal(blockListBytes, &blockList); err != nil {
		p.API.LogError("Unable to read kv", "error", err)
		return nil, errors.Wrapf(err, "Unable to read kv")
	}

	p.notifyListChanged(blockList)

	return blockList, nil
}

func (p *Plugin) handleClearSilencerList(userId string) string {
	if err := p.writeSilencerList(userId, []string{}); err != nil {
		return err.Error()
	}
	return "List cleared"
}

func (p *Plugin) handleSilencerList(userId string) string {
	blockList, err := p.readSilencerList(userId)
	if err != nil || len(blockList) == 0 {
		return "###### You have no silenced users\n"
	}
	result := "###### Users you've silenced:\n"
	if users, err := p.API.GetUsersByUsernames(blockList); err != nil {
		p.API.LogError("Unable to get users in list", "error", err.Error())
		return "Unable to fetch silencer list"
	} else {
		for _, user := range users {
			result += fmt.Sprintf("@%v\n", user.Username)
		}
	}
	return result
}

func (p *Plugin) handleToggleSilencer(senderID string, blockUsername string) string {
	user, appErr := p.API.GetUser(senderID)
	if appErr != nil {
		p.API.LogError("Unable to get user", "error", appErr.Error())
		return "Cannot get user"
	}

	blockUser, appErr := p.API.GetUserByUsername(blockUsername)
	if appErr != nil {
		p.API.LogError("Unable to get user err", "error", appErr.Error())
		return "Cannot get the other user"
	}

	blockList, err := p.readSilencerList(senderID)
	if err != nil {
		return err.Error()
	}

	found := -1
	for idx, userName := range blockList {
		if userName == blockUser.Username {
			found = idx
			break
		}
	}

	result := "@%v asked @%v to be quiet"

	if found == -1 {
		blockList = append(blockList, blockUser.Username)
	} else {
		blockList = append(blockList[:found], blockList[found+1:]...)
		result = "@%v allowed @%v to speak"
	}

	if err := p.writeSilencerList(senderID, blockList); err != nil {
		return err.Error()
	}

	return fmt.Sprintf(result, user.Username, blockUser.Username)
}

func (p *Plugin) handleCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	fields := strings.Fields(args.Command)
	command := ""
	if len(fields) == 2 {
		command = fields[1]
	}
	switch command {
	case "help":
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         commandHelp,
		}, nil
	case "clear":
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         p.handleClearSilencerList(args.UserId),
		}, nil
	case "":
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         p.handleSilencerList(args.UserId),
		}, nil
	default:
		if command[0] == '@' {
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         p.handleToggleSilencer(args.UserId, command[1:]),
			}, nil
		}
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Unknown command: " + command),
		}, nil
	}
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	if trigger == commandTrigger {
		return p.handleCommand(c, args)
	}
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf("Unknown command: " + args.Command),
	}, nil
}

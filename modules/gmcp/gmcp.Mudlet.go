package gmcp

import (
	"embed"
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/plugins"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

var (
	//go:embed files/*
	files embed.FS
)

// MudletConfig holds the configuration for Mudlet clients
type MudletConfig struct {
	// Mapper configuration
	MapperVersion string `json:"mapper_version" yaml:"mapper_version"`
	MapperURL     string `json:"mapper_url" yaml:"mapper_url"`

	// UI configuration
	UIVersion string `json:"ui_version" yaml:"ui_version"`
	UIURL     string `json:"ui_url" yaml:"ui_url"`

	// Map data configuration
	MapVersion string `json:"map_version" yaml:"map_version"`
	MapURL     string `json:"map_url" yaml:"map_url"`

	// Discord Rich Presence configuration
	DiscordApplicationID string `json:"discord_application_id" yaml:"discord_application_id"`
	DiscordInviteURL     string `json:"discord_invite_url" yaml:"discord_invite_url"`
	DiscordLargeImageKey string `json:"discord_large_image_key" yaml:"discord_large_image_key"`
	DiscordDetails       string `json:"discord_details" yaml:"discord_details"`
	DiscordState         string `json:"discord_state" yaml:"discord_state"`
	DiscordGame          string `json:"discord_game" yaml:"discord_game"`
	DiscordSmallImageKey string `json:"discord_small_image_key" yaml:"discord_small_image_key"`
}

// GMCPMudletModule handles Mudlet-specific GMCP functionality
type GMCPMudletModule struct {
	plug        *plugins.Plugin
	config      MudletConfig
	mudletUsers map[int]bool // Track which users are using Mudlet clients
}

// GMCPMudletDetected is an event fired when a Mudlet client is detected
type GMCPMudletDetected struct {
	ConnectionId uint64
	UserId       int
}

func (g GMCPMudletDetected) Type() string { return `GMCPMudletDetected` }

// GMCPDiscordStatusRequest is an event fired when a client requests Discord status information
type GMCPDiscordStatusRequest struct {
	UserId int
}

func (g GMCPDiscordStatusRequest) Type() string { return `GMCPDiscordStatusRequest` }

// GMCPDiscordMessage is an event fired when a client sends a Discord-related GMCP message
type GMCPDiscordMessage struct {
	ConnectionId uint64
	Command      string
	Payload      []byte
}

func (g GMCPDiscordMessage) Type() string { return `GMCPDiscordMessage` }

func init() {
	// Set up a default configuration first
	g := GMCPMudletModule{
		plug: plugins.New(`gmcp.Mudlet`, `1.0`),
		config: MudletConfig{
			MapperVersion: "1",                                                                                         // Default value
			MapperURL:     "https://github.com/GoMudEngine/MudletMapper/releases/latest/download/GoMudMapper.mpackage", // Default value
			UIVersion:     "1",                                                                                         // Default value
			UIURL:         "https://github.com/GoMudEngine/MudletUI/releases/latest/download/GoMudUI.mpackage",         // Default value
			MapVersion:    "1",                                                                                         // Default value
			MapURL:        "https://github.com/GoMudEngine/MudletMapper/releases/latest/download/gomud.dat",            // Default value
			// Discord defaults
			DiscordApplicationID: "1298377884154724412",           // Default value
			DiscordInviteURL:     "https://discord.gg/FaauSYej3n", // Default value
			DiscordLargeImageKey: "server-icon",                   // Default value
			DiscordDetails:       "Playing on GoMud",              // Default value
			DiscordState:         "Exploring the world",           // Default value
			DiscordGame:          "GoMud",                         // Default value
			DiscordSmallImageKey: "character-icon",                // Default value
		},
		mudletUsers: make(map[int]bool), // Initialize the mudletUsers map
	}

	// Attach embedded filesystem without logging errors
	_ = g.plug.AttachFileSystem(files)

	// Load config values from plugin config system
	if mapperVersion, ok := g.plug.Config.Get(`MapperVersion`).(string); ok {
		g.config.MapperVersion = mapperVersion
	}
	if mapperURL, ok := g.plug.Config.Get(`MapperURL`).(string); ok {
		g.config.MapperURL = mapperURL
	}
	if uiVersion, ok := g.plug.Config.Get(`UIVersion`).(string); ok {
		g.config.UIVersion = uiVersion
	}
	if uiURL, ok := g.plug.Config.Get(`UIURL`).(string); ok {
		g.config.UIURL = uiURL
	}
	if mapVersion, ok := g.plug.Config.Get(`MapVersion`).(string); ok {
		g.config.MapVersion = mapVersion
	}
	if mapURL, ok := g.plug.Config.Get(`MapURL`).(string); ok {
		g.config.MapURL = mapURL
	}
	if discordAppID, ok := g.plug.Config.Get(`DiscordApplicationID`).(string); ok {
		g.config.DiscordApplicationID = discordAppID
	}
	if discordInviteURL, ok := g.plug.Config.Get(`DiscordInviteURL`).(string); ok {
		g.config.DiscordInviteURL = discordInviteURL
	}
	if discordLargeImageKey, ok := g.plug.Config.Get(`DiscordLargeImageKey`).(string); ok {
		g.config.DiscordLargeImageKey = discordLargeImageKey
	}
	if discordDetails, ok := g.plug.Config.Get(`DiscordDetails`).(string); ok {
		g.config.DiscordDetails = discordDetails
	}
	if discordState, ok := g.plug.Config.Get(`DiscordState`).(string); ok {
		g.config.DiscordState = discordState
	}
	if discordGame, ok := g.plug.Config.Get(`DiscordGame`).(string); ok {
		g.config.DiscordGame = discordGame
	}
	if discordSmallImageKey, ok := g.plug.Config.Get(`DiscordSmallImageKey`).(string); ok {
		g.config.DiscordSmallImageKey = discordSmallImageKey
	}

	// Register event listeners
	events.RegisterListener(events.PlayerSpawn{}, g.playerSpawnHandler)
	events.RegisterListener(GMCPMudletDetected{}, g.mudletDetectedHandler)
	events.RegisterListener(GMCPDiscordStatusRequest{}, g.discordStatusRequestHandler)
	events.RegisterListener(GMCPDiscordMessage{}, g.discordMessageHandler)
	events.RegisterListener(events.RoomChange{}, g.roomChangeHandler)

	// Register the Mudlet-specific user commands - set as hidden (true for first bool)
	g.plug.AddUserCommand("mudletmap", g.sendMapCommand, true, false)
	g.plug.AddUserCommand("mudletui", g.sendUICommand, false, false)
	g.plug.AddUserCommand("checkclient", g.checkClientCommand, true, false)
}

// sendUICommand is a user command that sends UI-related GMCP messages to Mudlet clients
func (g *GMCPMudletModule) sendUICommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Only send if the client is Mudlet
	connId := user.ConnectionId()
	if gmcpData, ok := gmcpModule.cache.Get(connId); ok && gmcpData.Client.IsMudlet {
		// Process command arguments
		args := strings.Fields(rest)
		if len(args) > 0 {
			switch args[0] {
			case "install":
				// Send UI install message
				g.sendMudletUIInstall(user.UserId)
				user.SendText("\n<ansi fg=\"green\">UI installation package sent to your Mudlet client.</ansi> If it doesn't install automatically, you may need to accept the installation prompt in Mudlet.\n")
				// Set a flag to prevent the checkclient message from showing again
				user.SetConfigOption("mudlet_ui_prompt_disabled", true)
			case "remove":
				// Send UI remove message
				g.sendMudletUIRemove(user.UserId)
				user.SendText("\n<ansi fg=\"yellow\">UI removal command sent to your Mudlet client.</ansi>\n")
			case "update":
				// Send UI update message
				g.sendMudletUIUpdate(user.UserId)
				user.SendText("\n<ansi fg=\"cyan\">Manual UI update check sent to your Mudlet client.</ansi>\n")
			case "hide":
				// Set a flag to prevent the checkclient message from showing again
				user.SetConfigOption("mudlet_ui_prompt_disabled", true)
				user.SendText("\n<ansi fg=\"green\">The Mudlet UI prompt has been hidden.</ansi> You won't see these messages again when logging in.\n")
				user.SendText("You can use <ansi fg=\"command\">mudletui show</ansi> in the future if you want to see the prompts again.\n")
			case "show":
				// Remove the flag to allow the checkclient message to show again
				user.SetConfigOption("mudlet_ui_prompt_disabled", false)
				user.SendText("\n<ansi fg=\"green\">The Mudlet UI prompt has been re-enabled.</ansi> You'll see these messages again when logging in.\n")
				user.SendText("You can use <ansi fg=\"command\">mudletui hide</ansi> in the future if you want to hide the prompts again.\n")
			default:
				// Unknown command
				user.SendText("\nUsage: mudletui install|remove|update|hide|show\n\nType '<ansi fg=\"command\">help mudletui</ansi>' for more information.\n")
			}
		} else {
			// No arguments provided - show status and available commands
			mudName := configs.GetServerConfig().MudName.String()

			// Check current status of prompt display
			promptDisabled := user.GetConfigOption("mudlet_ui_prompt_disabled")
			promptStatus := "<ansi fg=\"green\">ENABLED</ansi>"
			if promptDisabled != nil && promptDisabled.(bool) {
				promptStatus = "<ansi fg=\"red\">HIDDEN</ansi>"
			}

			user.SendText("\n<ansi fg=\"cyan-bold\">" + mudName + " Mudlet UI Management</ansi>\n")
			user.SendText("<ansi fg=\"yellow-bold\">Status:</ansi>\n")
			user.SendText("  Login message display: " + promptStatus + "\n")
			user.SendText("<ansi fg=\"yellow-bold\">Available Commands:</ansi>\n")
			user.SendText("  <ansi fg=\"command\">mudletui install</ansi> - Install the Mudlet UI package\n")
			user.SendText("  <ansi fg=\"command\">mudletui remove</ansi>  - Remove the Mudlet UI package\n")
			user.SendText("  <ansi fg=\"command\">mudletui update</ansi>  - Manually check for updates to the Mudlet UI package\n")
			user.SendText("  <ansi fg=\"command\">mudletui hide</ansi>    - Hide login messages\n")
			user.SendText("  <ansi fg=\"command\">mudletui show</ansi>    - Enable login messages\n\n")
			user.SendText("For more information, type <ansi fg=\"command\">help mudletui</ansi>\n")
		}

		// Return true to indicate the command was handled
		return true, nil
	} else {
		// Client is not Mudlet
		user.SendText("\n<ansi fg=\"red\">This command is only available for Mudlet clients.</ansi> You are currently using: " + gmcpData.Client.Name + "\n")
	}

	// Command was handled
	return true, nil
}

// sendMudletUIInstall sends the UI installation GMCP message
func (g *GMCPMudletModule) sendMudletUIInstall(userId int) {
	if userId < 1 {
		return
	}

	// Create a payload for UI installation
	payload := struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}{
		Version: g.config.UIVersion,
		URL:     g.config.UIURL,
	}

	// Send the Client.GUI message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "Client.GUI",
		Payload: payload,
	})

	mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Mudlet UI install config", "userId", userId)
}

// sendMudletUIRemove sends the UI remove GMCP message
func (g *GMCPMudletModule) sendMudletUIRemove(userId int) {
	if userId < 1 {
		return
	}

	// Create a payload for UI removal
	payload := struct {
		GoMudUI string `json:"gomudui"`
	}{
		GoMudUI: "remove",
	}

	// Send the Client.GUI message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "Client.GUI",
		Payload: payload,
	})

	mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Mudlet UI remove command", "userId", userId)
}

// sendMudletUIUpdate sends the UI update GMCP message
func (g *GMCPMudletModule) sendMudletUIUpdate(userId int) {
	if userId < 1 {
		return
	}

	// Create a payload for UI update
	payload := struct {
		GoMudUI string `json:"gomudui"`
	}{
		GoMudUI: "update",
	}

	// Send the Client.GUI message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "Client.GUI",
		Payload: payload,
	})

	mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Mudlet UI update command", "userId", userId)
}

// sendMapCommand is a user command that sends the map URL to Mudlet clients
func (g *GMCPMudletModule) sendMapCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Only send if the client is Mudlet
	connId := user.ConnectionId()
	if gmcpData, ok := gmcpModule.cache.Get(connId); ok && gmcpData.Client.IsMudlet {
		// Send the map URL
		g.sendMudletMapConfig(user.UserId)

		// Return true to indicate the command was handled (but don't show any output to the user)
		return true, nil
	}

	// Return false to indicate the command wasn't handled (if not a Mudlet client)
	// This allows other handlers to potentially process it
	return false, nil
}

// sendMudletMapConfig sends the Mudlet map configuration via GMCP
func (g *GMCPMudletModule) sendMudletMapConfig(userId int) {
	if userId < 1 {
		return
	}

	// Create a payload for the Client.Map message
	mapConfig := map[string]string{
		"url": g.config.MapURL,
	}

	// Send the Client.Map message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "Client.Map",
		Payload: mapConfig,
	})

	mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Mudlet map config", "userId", userId)
}

// playerSpawnHandler sends Mudlet-specific GMCP when a player connects
func (g *GMCPMudletModule) playerSpawnHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.PlayerSpawn)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "PlayerSpawn", "Actual Type", e.Type())
		return events.Cancel
	}

	// Check if the client is Mudlet
	if gmcpData, ok := gmcpModule.cache.Get(evt.ConnectionId); ok {
		if gmcpData.Client.IsMudlet {
			// Send Mudlet-specific GMCP
			g.sendMudletConfig(evt.UserId)
		}
	}

	return events.Continue
}

// mudletDetectedHandler handles the event when a Mudlet client is detected
func (g *GMCPMudletModule) mudletDetectedHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(GMCPMudletDetected)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "GMCPMudletDetected", "Actual Type", e.Type())
		return events.Cancel
	}

	if evt.UserId > 0 {
		g.sendMudletConfig(evt.UserId)
	}

	return events.Continue
}

// sendMudletConfig sends the Mudlet configuration via GMCP
func (g *GMCPMudletModule) sendMudletConfig(userId int) {
	if userId < 1 {
		return
	}

	// Create a GUI payload with mapper version and url
	guiPayload := struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}{
		Version: g.config.MapperVersion,
		URL:     g.config.MapperURL,
	}

	// Send the Client.GUI message with mapper version and URL
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "Client.GUI",
		Payload: guiPayload,
	})

	// Get the user record to access character details
	user := users.GetByUserId(userId)
	if user == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get user record for Discord info", "userId", userId)
		return
	}

	// Create a payload for Discord.Info - only applicationid and inviteurl
	discordInfoPayload := struct {
		ApplicationID string `json:"applicationid"`
		InviteURL     string `json:"inviteurl"`
	}{
		ApplicationID: g.config.DiscordApplicationID,
		InviteURL:     g.config.DiscordInviteURL,
	}

	// Send the External.Discord.Info message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "External.Discord.Info",
		Payload: discordInfoPayload,
	})

	// Send the Discord Status information
	g.sendDiscordStatus(userId)

	mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Mudlet package config and Discord info", "userId", userId)
}

// checkClientCommand checks if the player is using Mudlet and shows information if they are
func (g *GMCPMudletModule) checkClientCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Get the connection ID and check if the client is Mudlet
	connId := user.ConnectionId()
	if gmcpData, ok := gmcpModule.cache.Get(connId); ok && gmcpData.Client.IsMudlet {
		// Check if the user has disabled the prompt
		promptDisabled := user.GetConfigOption("mudlet_ui_prompt_disabled")
		if promptDisabled != nil && promptDisabled.(bool) {
			// User has disabled the prompt, so don't show the message
			return true, nil
		}

		// Show a brief intro message
		user.SendText("\n\n<ansi fg=\"cyan-bold\">We have detected you are using Mudlet as a client.</ansi>\n")

		// Use the standard help system to show the mudletui help
		usercommands.Help("mudletui", user, room, flags)

		// Command was handled
		return true, nil
	}

	// Client is not Mudlet - return true but don't show any message
	// (Return true anyway to avoid command showing up in help)
	return true, nil
}

// discordStatusRequestHandler handles the GMCPDiscordStatusRequest event
func (g *GMCPMudletModule) discordStatusRequestHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(GMCPDiscordStatusRequest)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "GMCPDiscordStatusRequest", "Actual Type", e.Type())
		return events.Cancel
	}

	// Get the user record to access character details
	userId := evt.UserId
	user := users.GetByUserId(userId)
	if user == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get user record for Discord info/status", "userId", userId)
		return events.Cancel
	}

	// Create a payload for Discord.Info - only applicationid and inviteurl
	discordInfoPayload := struct {
		ApplicationID string `json:"applicationid"`
		InviteURL     string `json:"inviteurl"`
	}{
		ApplicationID: g.config.DiscordApplicationID,
		InviteURL:     g.config.DiscordInviteURL,
	}

	// Send the External.Discord.Info message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "External.Discord.Info",
		Payload: discordInfoPayload,
	})

	// Also send Discord Status information
	g.sendDiscordStatus(userId)

	mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Discord Info and Status in response to request", "userId", userId)
	return events.Continue
}

// discordMessageHandler handles Discord-related GMCP messages from clients
func (g *GMCPMudletModule) discordMessageHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(GMCPDiscordMessage)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "GMCPDiscordMessage", "Actual Type", e.Type())
		return events.Cancel
	}

	// Try to find the user ID associated with this connection
	userId := 0
	for _, user := range users.GetAllActiveUsers() {
		if user.ConnectionId() == evt.ConnectionId {
			userId = user.UserId
			break
		}
	}

	if userId == 0 {
		// No user associated with this connection
		return events.Cancel
	}

	// Log the message for debugging
	mudlog.Info("Mudlet GMCP Discord", "type", evt.Command, "userId", userId, "payload", string(evt.Payload))

	switch evt.Command {
	case "Hello":
		// Only send Discord.Info on Hello, as we don't have character info yet
		discordInfoPayload := struct {
			ApplicationID string `json:"applicationid"`
			InviteURL     string `json:"inviteurl"`
		}{
			ApplicationID: g.config.DiscordApplicationID,
			InviteURL:     g.config.DiscordInviteURL,
		}

		// Send the External.Discord.Info message
		events.AddToQueue(GMCPOut{
			UserId:  userId,
			Module:  "External.Discord.Info",
			Payload: discordInfoPayload,
		})

		mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Discord Info in response to Hello", "userId", userId)
	case "Get":
		// Get the user record to check if we have character info
		user := users.GetByUserId(userId)
		if user == nil || user.Character == nil {
			// If we don't have character info yet, don't send anything
			mudlog.Debug("GMCP", "type", "Mudlet", "action", "Ignoring Discord.Get request (no character info yet)", "userId", userId)
			return events.Continue
		}

		// We have character info, so send Discord.Status
		g.sendDiscordStatus(userId)
		mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Discord Status in response to Get", "userId", userId)
	case "Status":
		// Client sent a status update - just log it for now
		// No specific handling needed
	}

	return events.Continue
}

// roomChangeHandler handles the RoomChange event to update Discord status when players change areas/zones
func (g *GMCPMudletModule) roomChangeHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.RoomChange)
	if !typeOk {
		return events.Cancel
	}

	// Only handle player movements (not mobs)
	if evt.UserId == 0 || evt.MobInstanceId > 0 {
		return events.Continue
	}

	// Check if this is a Mudlet client
	isMudletUser := false
	if known, exists := g.mudletUsers[evt.UserId]; exists && known {
		isMudletUser = true
	} else if g.isMudletClient(evt.UserId) {
		g.mudletUsers[evt.UserId] = true
		isMudletUser = true
	}

	if !isMudletUser {
		return events.Continue
	}

	// Load rooms and check for zone change
	oldRoom := rooms.LoadRoom(evt.FromRoomId)
	newRoom := rooms.LoadRoom(evt.ToRoomId)
	if oldRoom == nil || newRoom == nil {
		return events.Continue
	}

	// Update Discord status on zone change
	if oldRoom.Zone != newRoom.Zone {
		g.sendDiscordStatus(evt.UserId)
	}

	return events.Continue
}

// sendDiscordStatus sends the current Discord status information to the client
func (g *GMCPMudletModule) sendDiscordStatus(userId int) {
	if userId < 1 {
		return
	}

	// Get the user record to access character details
	user := users.GetByUserId(userId)
	if user == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get user record for Discord status", "userId", userId)
		return
	}

	// Get the current room
	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get room for Discord status", "userId", userId, "roomId", user.Character.RoomId)
		return
	}

	// Create a payload for Discord.Status
	discordStatusPayload := struct {
		Details       string `json:"details"`
		State         string `json:"state"`
		Game          string `json:"game"`
		LargeImageKey string `json:"large_image_key"`
		SmallImageKey string `json:"small_image_key"`
		PartySize     int    `json:"partysize"`
		PartyMax      int    `json:"partymax"`
	}{
		Details:       g.config.DiscordDetails,
		State:         fmt.Sprintf("Exploring %s", room.Zone),
		Game:          g.config.DiscordGame,
		LargeImageKey: g.config.DiscordLargeImageKey,
		SmallImageKey: g.config.DiscordSmallImageKey,
		PartySize:     0,
		PartyMax:      10,
	}

	// Check if the user is in a party
	if party := parties.Get(userId); party != nil {
		// Only set party size if there are 2 or more members
		if len(party.GetMembers()) >= 2 {
			discordStatusPayload.PartySize = len(party.GetMembers())
		}
	}

	// Send the External.Discord.Status message
	events.AddToQueue(GMCPOut{
		UserId:  userId,
		Module:  "External.Discord.Status",
		Payload: discordStatusPayload,
	})

	mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Discord status update", "userId", userId, "zone", room.Zone)
}

// isMudletClient checks if the given user ID is using a Mudlet client
func (g *GMCPMudletModule) isMudletClient(userId int) bool {
	if userId < 1 {
		return false
	}

	// First check our cache of known Mudlet users
	if known, exists := g.mudletUsers[userId]; exists {
		return known
	}

	// If not in cache, check the connection
	connId := users.GetConnectionId(userId)
	if connId == 0 {
		return false
	}

	// Check the cache to see if this is a Mudlet client
	if gmcpData, ok := gmcpModule.cache.Get(connId); ok && gmcpData.Client.IsMudlet {
		// Store for future reference
		g.mudletUsers[userId] = true
		return true
	}

	return false
}

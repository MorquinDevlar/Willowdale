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

// Default configuration values
var defaultConfig = MudletConfig{
	// Mapper configuration
	MapperVersion: "1",
	MapperURL:     "https://github.com/GoMudEngine/MudletMapper/releases/latest/download/GoMudMapper.mpackage",

	// UI configuration
	UIVersion: "1",
	UIURL:     "https://github.com/GoMudEngine/MudletUI/releases/latest/download/GoMudUI.mpackage",

	// Map data configuration
	MapVersion: "1",
	MapURL:     "https://github.com/GoMudEngine/MudletMapper/releases/latest/download/gomud.dat",

	// Discord Rich Presence configuration
	DiscordApplicationID: "1298377884154724412",           // Default GoMud Discord Server
	DiscordInviteURL:     "https://discord.gg/FaauSYej3n", // Default GoMud Discord Server
	DiscordDetails:       "Using GoMudEngine",
	DiscordState:         "Exploring the world",
	DiscordLargeImageKey: "server-icon",
	DiscordSmallImageKey: "character-icon",
}

func init() {
	// Create module with basic structure
	g := GMCPMudletModule{
		plug:        plugins.New(`gmcp.Mudlet`, `1.0`),
		mudletUsers: make(map[int]bool),
	}

	// Attach filesystem with proper error handling
	if err := g.plug.AttachFileSystem(files); err != nil {
		panic(err)
	}

	// Register callbacks for load/save
	g.plug.Callbacks.SetOnLoad(g.load)
	g.plug.Callbacks.SetOnSave(g.save)

	// Register event listeners
	events.RegisterListener(events.PlayerSpawn{}, g.playerSpawnHandler)
	events.RegisterListener(GMCPMudletDetected{}, g.mudletDetectedHandler)
	events.RegisterListener(GMCPDiscordStatusRequest{}, g.discordStatusRequestHandler)
	events.RegisterListener(GMCPDiscordMessage{}, g.discordMessageHandler)
	events.RegisterListener(events.RoomChange{}, g.roomChangeHandler)
	events.RegisterListener(events.PartyUpdated{}, g.partyUpdateHandler)

	// Register the Mudlet-specific user commands
	g.plug.AddUserCommand("mudletmap", g.sendMapCommand, true, false)
	g.plug.AddUserCommand("mudletui", g.sendUICommand, false, false)
	g.plug.AddUserCommand("checkclient", g.checkClientCommand, true, false)
	g.plug.AddUserCommand("discord", g.discordCommand, true, false)
}

// load handles loading configuration from the plugin's storage
func (g *GMCPMudletModule) load() {
	// Start with default values
	g.config = defaultConfig

	// Load individual config values
	if mapperVersion, ok := g.plug.Config.Get(`mapper_version`).(string); ok {
		g.config.MapperVersion = mapperVersion
	}
	if mapperURL, ok := g.plug.Config.Get(`mapper_url`).(string); ok {
		g.config.MapperURL = mapperURL
	}
	if uiVersion, ok := g.plug.Config.Get(`ui_version`).(string); ok {
		g.config.UIVersion = uiVersion
	}
	if uiURL, ok := g.plug.Config.Get(`ui_url`).(string); ok {
		g.config.UIURL = uiURL
	}
	if mapVersion, ok := g.plug.Config.Get(`map_version`).(string); ok {
		g.config.MapVersion = mapVersion
	}
	if mapURL, ok := g.plug.Config.Get(`map_url`).(string); ok {
		g.config.MapURL = mapURL
	}
	if discordAppID, ok := g.plug.Config.Get(`discord_application_id`).(string); ok {
		g.config.DiscordApplicationID = discordAppID
	}
	if discordInviteURL, ok := g.plug.Config.Get(`discord_invite_url`).(string); ok {
		g.config.DiscordInviteURL = discordInviteURL
	}
	if discordLargeImageKey, ok := g.plug.Config.Get(`discord_large_image_key`).(string); ok {
		g.config.DiscordLargeImageKey = discordLargeImageKey
	}
	if discordDetails, ok := g.plug.Config.Get(`discord_details`).(string); ok {
		g.config.DiscordDetails = discordDetails
	}
	if discordState, ok := g.plug.Config.Get(`discord_state`).(string); ok {
		g.config.DiscordState = discordState
	}
	if discordSmallImageKey, ok := g.plug.Config.Get(`discord_small_image_key`).(string); ok {
		g.config.DiscordSmallImageKey = discordSmallImageKey
	}
}

// save handles saving configuration to the plugin's storage
func (g *GMCPMudletModule) save() {
	g.plug.WriteStruct(`mudlet_config`, g.config)
}

// getMapperConfig returns the mapper configuration with defaults
func (g *GMCPMudletModule) getMapperConfig() (version, url string) {
	// Start with default values from defaultConfig
	version = defaultConfig.MapperVersion
	url = defaultConfig.MapperURL

	// Check for overrides in plugin config
	if v, ok := g.plug.Config.Get(`mapper_version`).(string); ok {
		version = v
	}
	if u, ok := g.plug.Config.Get(`mapper_url`).(string); ok {
		url = u
	}
	return
}

// getUIConfig returns the UI configuration with defaults
func (g *GMCPMudletModule) getUIConfig() (version, url string) {
	// Start with default values from defaultConfig
	version = defaultConfig.UIVersion
	url = defaultConfig.UIURL

	// Check for overrides in plugin config
	if v, ok := g.plug.Config.Get(`ui_version`).(string); ok {
		version = v
	}
	if u, ok := g.plug.Config.Get(`ui_url`).(string); ok {
		url = u
	}
	return
}

// getMapConfig returns the map configuration with defaults
func (g *GMCPMudletModule) getMapConfig() (version, url string) {
	// Start with default values from defaultConfig
	version = defaultConfig.MapVersion
	url = defaultConfig.MapURL

	// Check for overrides in plugin config
	if v, ok := g.plug.Config.Get(`map_version`).(string); ok {
		version = v
	}
	if u, ok := g.plug.Config.Get(`map_url`).(string); ok {
		url = u
	}
	return
}

// getDiscordConfig returns the Discord configuration with defaults
func (g *GMCPMudletModule) getDiscordConfig() (appID, inviteURL, largeImageKey, details, state, game, smallImageKey string) {
	// Start with default values from defaultConfig
	appID = defaultConfig.DiscordApplicationID
	inviteURL = defaultConfig.DiscordInviteURL
	largeImageKey = defaultConfig.DiscordLargeImageKey
	details = defaultConfig.DiscordDetails
	state = defaultConfig.DiscordState
	smallImageKey = defaultConfig.DiscordSmallImageKey

	// Get game name from server config - this is the source of truth
	game = configs.GetServerConfig().MudName.String()

	// Check for overrides in plugin config
	if id, ok := g.plug.Config.Get(`discord_application_id`).(string); ok {
		appID = id
	}
	if url, ok := g.plug.Config.Get(`discord_invite_url`).(string); ok {
		inviteURL = url
	}
	if key, ok := g.plug.Config.Get(`discord_large_image_key`).(string); ok {
		largeImageKey = key
	}
	if d, ok := g.plug.Config.Get(`discord_details`).(string); ok {
		details = d
	}
	if s, ok := g.plug.Config.Get(`discord_state`).(string); ok {
		state = s
	}
	if key, ok := g.plug.Config.Get(`discord_small_image_key`).(string); ok {
		smallImageKey = key
	}

	return
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

	// Get UI config values
	uiVersion, uiURL := g.getUIConfig()

	// Create a payload for UI installation
	payload := struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}{
		Version: uiVersion,
		URL:     uiURL,
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

	// Get map config values
	_, mapURL := g.getMapConfig()

	// Create a payload for the Client.Map message
	mapConfig := map[string]string{
		"url": mapURL,
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

	// Get mapper config values
	mapperVersion, mapperURL := g.getMapperConfig()

	// Create a GUI payload with mapper version and url
	guiPayload := struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}{
		Version: mapperVersion,
		URL:     mapperURL,
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

	// Check if Discord.Info is enabled
	enableInfo := user.GetConfigOption("discord_enable_info")
	if enableInfo == nil || enableInfo.(bool) {
		// Get Discord config values - only what we need for Discord.Info
		appID, inviteURL, largeImageKey, details, state, game, smallImageKey := g.getDiscordConfig()

		// Create a payload for Discord.Info - only applicationid and inviteurl
		discordInfoPayload := struct {
			ApplicationID string `json:"applicationid"`
			InviteURL     string `json:"inviteurl"`
		}{
			ApplicationID: appID,
			InviteURL:     inviteURL,
		}

		// Send the External.Discord.Info message
		events.AddToQueue(GMCPOut{
			UserId:  userId,
			Module:  "External.Discord.Info",
			Payload: discordInfoPayload,
		})

		// Send the Discord Status information with the config values we already have
		g.sendDiscordStatusWithConfig(userId, largeImageKey, details, state, game, smallImageKey)
	} else {
		mudlog.Debug("GMCP", "type", "Mudlet", "action", "Discord.Info package sending disabled for user", "userId", userId)
	}

	mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Mudlet package config", "userId", userId)
}

// sendDiscordStatusWithConfig sends the current Discord status information to the client using provided config values
func (g *GMCPMudletModule) sendDiscordStatusWithConfig(userId int, largeImageKey, details, state, game, smallImageKey string) {
	if userId < 1 {
		return
	}

	// Get the user record to access character details
	user := users.GetByUserId(userId)
	if user == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get user record for Discord status", "userId", userId)
		return
	}

	// Check if Discord.Status is enabled
	enableStatus := user.GetConfigOption("discord_enable_status")
	if enableStatus != nil && !enableStatus.(bool) {
		mudlog.Debug("GMCP", "type", "Mudlet", "action", "Discord.Status package sending disabled for user", "userId", userId)
		return
	}

	// Get the current room
	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get room for Discord status", "userId", userId, "roomId", user.Character.RoomId)
		return
	}

	// Check display preferences (default to true if not set)
	showArea := user.GetConfigOption("discord_show_area")
	if showArea == nil {
		showArea = true
	}
	showParty := user.GetConfigOption("discord_show_party")
	if showParty == nil {
		showParty = true
	}
	showName := user.GetConfigOption("discord_show_name")
	if showName == nil {
		showName = true
	}
	showLevel := user.GetConfigOption("discord_show_level")
	if showLevel == nil {
		showLevel = true
	}

	// Build the details string based on preferences
	detailsStr := details
	if showName.(bool) || showLevel.(bool) {
		detailsStr = ""
		if showName.(bool) {
			detailsStr = user.Character.Name
		}
		if showLevel.(bool) {
			if detailsStr != "" {
				detailsStr += " "
			}
			if showName.(bool) {
				detailsStr += fmt.Sprintf("(lvl. %d)", user.Character.Level)
			} else {
				detailsStr += fmt.Sprintf("Level %d", user.Character.Level)
			}
		}
	}

	// Create a payload for Discord.Status
	discordStatusPayload := struct {
		Details       string `json:"details"`
		State         string `json:"state"`
		Game          string `json:"game"`
		LargeImageKey string `json:"large_image_key"`
		SmallImageKey string `json:"small_image_key"`
		StartTime     int64  `json:"starttime"`
		PartySize     int    `json:"partysize,omitempty"`
		PartyMax      int    `json:"partymax,omitempty"`
	}{
		Details:       detailsStr,
		State:         "Exploring the world", // Default state
		Game:          game,
		LargeImageKey: largeImageKey,
		SmallImageKey: smallImageKey,
		StartTime:     user.GetConnectTime().Unix(),
	}

	// Only show area if enabled
	if showArea.(bool) {
		discordStatusPayload.State = fmt.Sprintf("Exploring %s", room.Zone)
	}

	// Check if the user is in a party
	if party := parties.Get(userId); party != nil {
		// Only show party info if enabled
		if showParty.(bool) {
			discordStatusPayload.PartySize = len(party.GetMembers())
			discordStatusPayload.PartyMax = 10
			if showArea.(bool) {
				discordStatusPayload.State = fmt.Sprintf("Group in %s", room.Zone)
			} else {
				discordStatusPayload.State = "In group"
			}
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

// sendDiscordStatus sends the current Discord status information to the client
func (g *GMCPMudletModule) sendDiscordStatus(userId int) {
	// Get Discord config values
	_, _, largeImageKey, details, state, game, smallImageKey := g.getDiscordConfig()
	g.sendDiscordStatusWithConfig(userId, largeImageKey, details, state, game, smallImageKey)
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

	// Get Discord config values once
	appID, inviteURL, largeImageKey, details, state, game, smallImageKey := g.getDiscordConfig()

	// Check if Discord.Info is enabled
	enableInfo := user.GetConfigOption("discord_enable_info")
	if enableInfo == nil || enableInfo.(bool) {
		// Create a payload for Discord.Info - only applicationid and inviteurl
		discordInfoPayload := struct {
			ApplicationID string `json:"applicationid"`
			InviteURL     string `json:"inviteurl"`
		}{
			ApplicationID: appID,
			InviteURL:     inviteURL,
		}

		// Send the External.Discord.Info message
		events.AddToQueue(GMCPOut{
			UserId:  userId,
			Module:  "External.Discord.Info",
			Payload: discordInfoPayload,
		})

		mudlog.Debug("GMCP", "type", "Mudlet", "action", "Sent Discord Info in response to request", "userId", userId)
	} else {
		mudlog.Debug("GMCP", "type", "Mudlet", "action", "Discord.Info package sending disabled for user", "userId", userId)
	}

	// Also send Discord Status information with the config values we already have
	g.sendDiscordStatusWithConfig(userId, largeImageKey, details, state, game, smallImageKey)

	mudlog.Info("GMCP", "type", "Mudlet", "action", "Processed Discord status request", "userId", userId)
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

	// Get user record for checking settings
	user := users.GetByUserId(userId)
	if user == nil {
		mudlog.Error("GMCP", "type", "Mudlet", "action", "Failed to get user record for Discord message handling", "userId", userId)
		return events.Cancel
	}

	switch evt.Command {
	case "Hello":
		// Check if Discord.Info is enabled
		enableInfo := user.GetConfigOption("discord_enable_info")
		if enableInfo == nil || enableInfo.(bool) {
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
		} else {
			mudlog.Debug("GMCP", "type", "Mudlet", "action", "Discord.Info package sending disabled for user", "userId", userId)
		}
	case "Get":
		// Check if Discord.Status is enabled
		enableStatus := user.GetConfigOption("discord_enable_status")
		if enableStatus == nil || enableStatus.(bool) {
			// If we don't have character info yet, don't send anything
			if user.Character == nil {
				mudlog.Debug("GMCP", "type", "Mudlet", "action", "Ignoring Discord.Get request (no character info yet)", "userId", userId)
				return events.Continue
			}

			// We have character info, so send Discord.Status
			g.sendDiscordStatus(userId)
			mudlog.Info("GMCP", "type", "Mudlet", "action", "Sent Discord Status in response to Get", "userId", userId)
		} else {
			mudlog.Debug("GMCP", "type", "Mudlet", "action", "Discord.Status package sending disabled for user", "userId", userId)
		}
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

// partyUpdateHandler handles party membership changes and updates Discord status
func (g *GMCPMudletModule) partyUpdateHandler(e events.Event) events.ListenerReturn {
	evt, typeOk := e.(events.PartyUpdated)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "PartyUpdated", "Actual Type", e.Type())
		return events.Cancel
	}

	// Update Discord status for all users in the party
	for _, userId := range evt.UserIds {
		// Only send updates to Mudlet clients
		if g.isMudletClient(userId) {
			g.sendDiscordStatus(userId)
		}
	}

	return events.Continue
}

// showDiscordHelp displays the help text and current settings for the discord command
func (g *GMCPMudletModule) showDiscordHelp(user *users.UserRecord) {
	// Use the help system to show the discord help
	usercommands.Help("discord", user, nil, 0)

	// Show current settings
	user.SendText("\n<ansi fg=\"subtitle\">Current settings:</ansi>\n")

	enableInfo := user.GetConfigOption("discord_enable_info")
	if enableInfo == nil || enableInfo.(bool) {
		user.SendText("  Info:   <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Info:   <ansi fg=\"red\">DISABLED</ansi>")
	}

	enableStatus := user.GetConfigOption("discord_enable_status")
	if enableStatus == nil || enableStatus.(bool) {
		user.SendText("  Status: <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Status: <ansi fg=\"red\">DISABLED</ansi>")
	}

	user.SendText("")

	showArea := user.GetConfigOption("discord_show_area")
	if showArea == nil || showArea.(bool) {
		user.SendText("  Area:   <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Area:   <ansi fg=\"red\">DISABLED</ansi>")
	}

	showParty := user.GetConfigOption("discord_show_party")
	if showParty == nil || showParty.(bool) {
		user.SendText("  Party:  <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Party:  <ansi fg=\"red\">DISABLED</ansi>")
	}

	showName := user.GetConfigOption("discord_show_name")
	if showName == nil || showName.(bool) {
		user.SendText("  Name:   <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Name:   <ansi fg=\"red\">DISABLED</ansi>")
	}

	showLevel := user.GetConfigOption("discord_show_level")
	if showLevel == nil || showLevel.(bool) {
		user.SendText("  Level:  <ansi fg=\"green\">ENABLED</ansi>")
	} else {
		user.SendText("  Level:  <ansi fg=\"red\">DISABLED</ansi>")
	}

	user.SendText("\n")
}

// discordCommand handles the discord command for controlling Discord status display
func (g *GMCPMudletModule) discordCommand(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	// Only send if the client is Mudlet
	connId := user.ConnectionId()
	if gmcpData, ok := gmcpModule.cache.Get(connId); ok && gmcpData.Client.IsMudlet {
		// Process command arguments
		args := strings.Fields(rest)
		if len(args) > 0 {
			switch args[0] {
			case "area":
				if len(args) < 2 {
					user.SendText("\nUsage: discord area on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_show_area", true)
					user.SendText("\n<ansi fg=\"green\">Area display in Discord status enabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				case "off":
					user.SetConfigOption("discord_show_area", false)
					user.SendText("\n<ansi fg=\"yellow\">Area display in Discord status disabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				default:
					user.SendText("\nUsage: discord area on|off\n")
				}
			case "party":
				if len(args) < 2 {
					user.SendText("\nUsage: discord party on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_show_party", true)
					user.SendText("\n<ansi fg=\"green\">Party display in Discord status enabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				case "off":
					user.SetConfigOption("discord_show_party", false)
					user.SendText("\n<ansi fg=\"yellow\">Party display in Discord status disabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				default:
					user.SendText("\nUsage: discord party on|off\n")
				}
			case "name":
				if len(args) < 2 {
					user.SendText("\nUsage: discord name on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_show_name", true)
					user.SendText("\n<ansi fg=\"green\">Character name display in Discord status enabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				case "off":
					user.SetConfigOption("discord_show_name", false)
					user.SendText("\n<ansi fg=\"yellow\">Character name display in Discord status disabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				default:
					user.SendText("\nUsage: discord name on|off\n")
				}
			case "level":
				if len(args) < 2 {
					user.SendText("\nUsage: discord level on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_show_level", true)
					user.SendText("\n<ansi fg=\"green\">Level display in Discord status enabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				case "off":
					user.SetConfigOption("discord_show_level", false)
					user.SendText("\n<ansi fg=\"yellow\">Level display in Discord status disabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				default:
					user.SendText("\nUsage: discord level on|off\n")
				}
			case "info":
				if len(args) < 2 {
					user.SendText("\nUsage: discord info on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_enable_info", true)
					user.SendText("\n<ansi fg=\"green\">Discord.Info package sending enabled.</ansi>\n")

					// Send the Info package immediately if enabled
					appID, inviteURL, _, _, _, _, _ := g.getDiscordConfig()
					discordInfoPayload := struct {
						ApplicationID string `json:"applicationid"`
						InviteURL     string `json:"inviteurl"`
					}{
						ApplicationID: appID,
						InviteURL:     inviteURL,
					}

					events.AddToQueue(GMCPOut{
						UserId:  user.UserId,
						Module:  "External.Discord.Info",
						Payload: discordInfoPayload,
					})
				case "off":
					user.SetConfigOption("discord_enable_info", false)
					user.SendText("\n<ansi fg=\"yellow\">Discord.Info package sending disabled.</ansi>\n")

					// Send an empty Discord.Info package to reset client cache
					emptyInfoPayload := struct {
						ApplicationID string `json:"applicationid"`
						InviteURL     string `json:"inviteurl"`
					}{
						ApplicationID: "",
						InviteURL:     "",
					}

					events.AddToQueue(GMCPOut{
						UserId:  user.UserId,
						Module:  "External.Discord.Info",
						Payload: emptyInfoPayload,
					})
				default:
					user.SendText("\nUsage: discord info on|off\n")
				}
			case "status":
				if len(args) < 2 {
					user.SendText("\nUsage: discord status on|off\n")
					return true, nil
				}
				switch args[1] {
				case "on":
					user.SetConfigOption("discord_enable_status", true)
					user.SendText("\n<ansi fg=\"green\">Discord.Status package sending enabled.</ansi>\n")
					g.sendDiscordStatus(user.UserId)
				case "off":
					user.SetConfigOption("discord_enable_status", false)
					user.SendText("\n<ansi fg=\"yellow\">Discord.Status package sending disabled.</ansi>\n")

					// Send an empty Discord.Status package to reset client cache
					emptyStatusPayload := struct {
						Details       string `json:"details"`
						State         string `json:"state"`
						Game          string `json:"game"`
						LargeImageKey string `json:"large_image_key"`
						SmallImageKey string `json:"small_image_key"`
						StartTime     int64  `json:"starttime"`
						PartySize     int    `json:"partysize,omitempty"`
						PartyMax      int    `json:"partymax,omitempty"`
					}{
						Details:       "",
						State:         "",
						Game:          "",
						LargeImageKey: "",
						SmallImageKey: "",
						StartTime:     0,
					}

					events.AddToQueue(GMCPOut{
						UserId:  user.UserId,
						Module:  "External.Discord.Status",
						Payload: emptyStatusPayload,
					})
				default:
					user.SendText("\nUsage: discord status on|off\n")
				}
			default:
				// Show help for unknown commands
				g.showDiscordHelp(user)
			}
		} else {
			// No arguments provided - show help
			g.showDiscordHelp(user)
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

package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/signal"
	"syscall"
	_ "unsafe"
	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/v2ui"
	"x-ui/web"
	"x-ui/web/global"
	"x-ui/web/service"

	"github.com/op/go-logging"
)

func runWebServer() {
	log.Printf("%v %v", config.GetName(), config.GetVersion())

	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatal("unknown log level:", config.GetLogLevel())
	}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}

	var server *web.Server

	server = web.NewServer()
	global.SetWebServer(server)
	err = server.Start()
	if err != nil {
		log.Println(err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			err := server.Stop()
			if err != nil {
				logger.Warning("stop server err:", err)
			}
			server = web.NewServer()
			global.SetWebServer(server)
			err = server.Start()
			if err != nil {
				log.Println(err)
				return
			}
		default:
			err := server.Stop()
			if err != nil {
				return
			}
			return
		}
	}
}

func resetSetting() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("reset setting failed:", err)
	} else {
		fmt.Println("reset setting success")
	}
}

func showSetting(show bool) {
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println("get current port failed,error info:", err)
		}
		userService := service.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println("get current user info failed,error info:", err)
		}
		username := userModel.Username
		userpasswd := userModel.Password
		if (username == "") || (userpasswd == "") {
			fmt.Println("current username or password is empty")
		}
		fmt.Println("current panel settings as follows:")
		fmt.Println("username:", username)
		fmt.Println("userpasswd:", userpasswd)
		fmt.Println("port:", port)
	}
}

func updateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotenabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotenabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotenabled[%v] success", status)
		}
	}
	return
}

func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Info("updateTgbotSetting tgBotToken success")
		}
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("updateTgbotSetting tgBotRuntime[%s] success", tgBotRuntime)
		}
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Info("updateTgbotSetting tgBotChatid success")
		}
	}
}

func updateSetting(port int, username string, password string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("set port failed:", err)
		} else {
			fmt.Printf("set port %v success", port)
		}
	}
	if username != "" || password != "" {
		userService := service.UserService{}
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("set username and password failed:", err)
		} else {
			fmt.Println("set username and password success")
		}
	}
}

func migrateDb() {
	inboundService := service.InboundService{}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database...")
	inboundService.MigrationRequirements()
	inboundService.RemoveOrphanedTraffics()
	fmt.Println("Migration done!")
}

func removeSecret() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	userService := service.UserService{}
	err = userService.RemoveUserSecret()
	if err != nil {
		fmt.Println(err)
	}
	settingService := service.SettingService{}
	err = settingService.SetSecretStatus(false)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	var rootCmd = &cobra.Command{
		Use: "x-ui",
	}

	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the web server",
		Run: func(cmd *cobra.Command, args []string) {
			runWebServer()
		},
	}

	var migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from other/old x-ui",
		Run: func(cmd *cobra.Command, args []string) {
			migrateDb()
		},
	}

	var v2uiCmd = &cobra.Command{
		Use:   "v2-ui",
		Short: "Migrate from v2-ui",
		Run: func(cmd *cobra.Command, args []string) {
			dbPath, _ := cmd.Flags().GetString("db")
			err := v2ui.MigrateFromV2UI(dbPath)
			if err != nil {
				fmt.Println("migrate from v2-ui failed:", err)
			}
		},
	}

	v2uiCmd.Flags().String("db", fmt.Sprintf("%s/v2-ui.db", config.GetDBFolderPath()), "set v2-ui db file path")

	var settingCmd = &cobra.Command{
		Use:   "setting",
		Short: "Set settings",
	}

	var resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset all settings",
		Run: func(cmd *cobra.Command, args []string) {
			resetSetting()
		},
	}

	var showCmd = &cobra.Command{
		Use:   "show",
		Short: "Show current settings",
		Run: func(cmd *cobra.Command, args []string) {
			showSetting(true)
		},
	}

	var updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update settings",
		Run: func(cmd *cobra.Command, args []string) {
			port, _ := cmd.Flags().GetInt("port")
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			updateSetting(port, username, password)
		},
	}

	updateCmd.Flags().Int("port", 0, "set panel port")
	updateCmd.Flags().String("username", "", "set login username")
	updateCmd.Flags().String("password", "", "set login password")

	var tgbotCmd = &cobra.Command{
		Use:   "tgbot",
		Short: "Update telegram bot settings",
		Run: func(cmd *cobra.Command, args []string) {
			tgbottoken, _ := cmd.Flags().GetString("tgbottoken")
			tgbotchatid, _ := cmd.Flags().GetString("tgbotchatid")
			tgbotRuntime, _ := cmd.Flags().GetString("tgbotRuntime")
			enabletgbot, _ := cmd.Flags().GetBool("enabletgbot")
			remove_secret, _ := cmd.Flags().GetBool("remove_secret")

			if tgbottoken != "" || tgbotchatid != "" || tgbotRuntime != "" {
				updateTgbotSetting(tgbottoken, tgbotchatid, tgbotRuntime)
			}

			if remove_secret {
				removeSecret()
			}

			if enabletgbot {
				updateTgbotEnableSts(enabletgbot)
			}
		},
	}

	tgbotCmd.Flags().String("tgbottoken", "", "set telegram bot token")
	tgbotCmd.Flags().String("tgbotchatid", "", "set telegram bot chat id")
	tgbotCmd.Flags().String("tgbotRuntime", "", "set telegram bot cron time")
	tgbotCmd.Flags().Bool("enabletgbot", false, "enable telegram bot notify")
	tgbotCmd.Flags().Bool("remove_secret", false, "remove secret")

	settingCmd.AddCommand(resetCmd, showCmd, updateCmd, tgbotCmd)

	rootCmd.AddCommand(runCmd, migrateCmd, v2uiCmd, settingCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

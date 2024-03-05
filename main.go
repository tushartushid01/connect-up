package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/RemoteState/connect-up/docs"
	"github.com/RemoteState/connect-up/env"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/connect-up/server"
)

// @title        ConnectUp API
// @version      1.0
// @description  This is the main server handling the ConnectUp major operations

// @contact.name   ConnectUp
// @contact.email  gaurav.bhardwaj@remotestate.com

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @BasePath  /
func main() {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// logrus.SetFormatter(&logrus.JSONFormatter{})
	// logrus.SetReportCaller(true)

	srv := server.SrvInit()
	go srv.Start()

	if env.InKubeCluster() {
		if env.IsDev() {
			docs.SwaggerInfo.Schemes = []string{"https"}
			docs.SwaggerInfo.Host = "dev-api.connectup.com"
		} else {
			docs.SwaggerInfo.Schemes = []string{"https"}
			docs.SwaggerInfo.Host = "connectup-api.connectup.com"
		}
	} else {
		docs.SwaggerInfo.Schemes = []string{"http"}
		docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%v", os.Getenv("PORT"))
	}
	<-done
	logrus.Info("Graceful shutdown")
	srv.Stop()
}

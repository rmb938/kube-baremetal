package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/rmb938/kube-baremetal/pkg/agent/action"
)

type server struct {
	Address string
	Manager *Manager

	logger logr.Logger
}

func NewServer(address string, manager *Manager) *server {
	return &server{
		Address: address,
		Manager: manager,

		logger: ctrllog.Log.WithName("agent-server"),
	}
}

func (s *server) Run(stop <-chan struct{}) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		param := gin.LogFormatterParams{
			Request: c.Request,
			Keys:    c.Keys,
		}

		// Stop timer
		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

		param.BodySize = c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path

		keyValues := []interface{}{
			"status", c.Writer.Status(),
			"latency", param.TimeStamp.Sub(start),
			"client-ip", param.ClientIP,
			"method", param.Method,
			"path", param.Path,
		}

		if len(param.ErrorMessage) > 0 {
			keyValues = append(keyValues, "error", param.ErrorMessage)
		}

		if param.BodySize != -1 {
			keyValues = append(keyValues, "size", param.BodySize)
		}

		s.logger.Info("agent request", keyValues...)
	})

	r.POST("/image", s.image)
	r.GET("/status", s.status)

	srv := &http.Server{
		Addr:    s.Address,
		Handler: r,
	}

	httpListener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("error listening on %s: %v", s.Address, err)
	}
	defer httpListener.Close()

	err = s.Manager.SendReady()
	if err != nil {
		return fmt.Errorf("error sending ready action: %v", err)
	}

	go func() {
		s.logger.Info("Starting agent http server")

		if err := srv.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			s.logger.Error(err, "Error when serving agent server")
		}
	}()

	<-stop
	s.logger.Info("Stopping agent http server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

func (s *server) image(c *gin.Context) {
	input := &action.ImageRequest{}

	if err := c.ShouldBindJSON(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	imageAction := action.NewImageAction(
		input.ImageURL,
		input.DiskPath,
		input.MetadataContents,
		input.NetworkDataContents,
		input.UserDataContents,
	)

	doingAction := s.Manager.DoAction(imageAction)

	if doingAction == false {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot run multiple actions"})
		c.Abort()
		return
	}

	c.Status(http.StatusAccepted)
}

func (s *server) status(c *gin.Context) {
	actionStatus, err := s.Manager.CurrentStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error getting current status: %v", err)})
		c.Abort()
		return
	}

	// there is no current actionStatus so return no content
	if actionStatus == nil {
		c.Status(http.StatusNoContent)
		return
	}

	// action has no errors
	c.JSON(http.StatusOK, actionStatus)
}

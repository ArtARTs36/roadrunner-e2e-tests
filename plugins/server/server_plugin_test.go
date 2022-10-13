package server

import (
	"bytes"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/roadrunner-server/config/v3"
	endure "github.com/roadrunner-server/endure/pkg/container"
	httpPlugin "github.com/roadrunner-server/http/v3"
	"github.com/roadrunner-server/logger/v3"
	mock_logger "github.com/roadrunner-server/rr-e2e-tests/mock"
	"github.com/roadrunner-server/server/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAppPipes(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr.yaml"
	vp.Prefix = "rr"

	err = container.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	errCh, err := container.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	tt := time.NewTimer(time.Second * 5)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer tt.Stop()
		for {
			select {
			case e := <-errCh:
				assert.NoError(t, e.Error)
				assert.NoError(t, container.Stop())
				return
			case <-c:
				er := container.Stop()
				assert.NoError(t, er)
				return
			case <-tt.C:
				assert.NoError(t, container.Stop())
				return
			}
		}
	}()

	wg.Wait()
}

func TestAppPipesBigResp(t *testing.T) {
	cont, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-pipes-big-resp.yaml"
	vp.Prefix = "rr"

	rd, wr, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = wr

	err = cont.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo4{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = cont.Init()
	require.NoError(t, err)

	ch, err := cont.Serve()
	require.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 5)
	stopCh <- struct{}{}
	wg.Wait()

	time.Sleep(time.Second)
	_ = wr.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, rd)
	require.NoError(t, err)
	require.GreaterOrEqual(t, strings.Count(buf.String(), "A"), 64000)
}

func TestAppSockets(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-sockets.yaml"
	vp.Prefix = "rr"

	err = container.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo2{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	errCh, err := container.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// stop after 10 seconds
	tt := time.NewTicker(time.Second * 10)

	for {
		select {
		case e := <-errCh:
			assert.NoError(t, e.Error)
			assert.NoError(t, container.Stop())
			tt.Stop()
			return
		case <-c:
			er := container.Stop()
			tt.Stop()
			if er != nil {
				panic(er)
			}
			return
		case <-tt.C:
			tt.Stop()
			assert.NoError(t, container.Stop())
			return
		}
	}
}

func TestAppPipesException(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-script-err.yaml"
	vp.Prefix = "rr"

	err = container.RegisterAll(
		vp,
		&server.Plugin{},
		&logger.Plugin{},
		&httpPlugin.Plugin{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	errCh, err := container.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-errCh:
				assert.Error(t, e.Error)
				assert.Contains(t, e.Error.Error(), "validation failed on the message sent to STDOUT, see: https://roadrunner.dev/docs/known-issues-stdout-crc/2.x/en, invalid message: warning: some weird php error warning: some weird php error warning: some weird php error warning: some weird php error warning: some weird php error")
				return
			case <-c:
				er := container.Stop()
				assert.NoError(t, er)
				return
			}
		}
	}()

	wg.Wait()
}

func TestAppTCPOnInit(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-tcp-on-init.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	l, oLogger := mock_logger.ZapTestLogger(zap.DebugLevel)
	err = container.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	ch, err := container.Serve()
	require.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-sig:
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 0").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 1").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 2").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 3").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 4").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 5").Len())
}

func TestAppSocketsOnInit(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	if err != nil {
		t.Fatal(err)
	}
	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-sockets-on-init.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	l, oLogger := mock_logger.ZapTestLogger(zap.DebugLevel)
	err = container.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	ch, err := container.Serve()
	require.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-sig:
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 0\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 1\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 2\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 3\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 4\n").Len())
	require.Equal(t, 1, oLogger.FilterMessageSnippet("The number is: 5\n").Len())
}

func TestAppSocketsOnInitFastClose(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	if err != nil {
		t.Fatal(err)
	}
	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-sockets-on-init-fast-close.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	l, oLogger := mock_logger.ZapTestLogger(zap.DebugLevel)
	err = container.RegisterAll(
		l,
		&server.Plugin{},
		&Foo2{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	ch, err := container.Serve()
	require.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-sig:
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = container.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	time.Sleep(time.Second * 10)
	stopCh <- struct{}{}
	wg.Wait()

	require.Equal(t, 1, oLogger.FilterMessageSnippet("process wait").Len())
}

func TestAppTCP(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-tcp.yaml"
	vp.Prefix = "rr"

	err = container.RegisterAll(
		vp,
		&server.Plugin{},
		&Foo3{},
		&logger.Plugin{},
	)
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	errCh, err := container.Serve()
	require.NoError(t, err)

	// stop by CTRL+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// stop after 10 seconds
	tt := time.NewTicker(time.Second * 10)

	for {
		select {
		case e := <-errCh:
			assert.NoError(t, e.Error)
			assert.NoError(t, container.Stop())
			return
		case <-c:
			er := container.Stop()
			if er != nil {
				panic(er)
			}
			return
		case <-tt.C:
			tt.Stop()
			assert.NoError(t, container.Stop())
			return
		}
	}
}

func TestAppWrongConfig(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)
	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rrrrrrrrrr.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	require.Error(t, container.Init())
}

func TestAppWrongRelay(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-wrong-relay.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.Error(t, err)

	_, err = container.Serve()
	require.Error(t, err)

	_ = container.Stop()
}

func TestAppWrongCommand(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-wrong-command.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

func TestAppWrongCommandOnInit(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-wrong-command-on-init.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

func TestAppNoAppSectionInConfig(t *testing.T) {
	container, err := endure.NewContainer(nil, endure.SetLogLevel(endure.ErrorLevel))
	require.NoError(t, err)

	// config plugin
	vp := &config.Plugin{}
	vp.Path = "configs/.rr-wrong-command.yaml"
	vp.Prefix = "rr"
	err = container.Register(vp)
	require.NoError(t, err)

	err = container.Register(&server.Plugin{})
	require.NoError(t, err)

	err = container.Register(&Foo3{})
	require.NoError(t, err)

	err = container.Register(&logger.Plugin{})
	require.NoError(t, err)

	err = container.Init()
	require.NoError(t, err)

	_, err = container.Serve()
	require.Error(t, err)
}

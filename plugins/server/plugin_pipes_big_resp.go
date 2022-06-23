package server

import (
	"context"
	"time"

	"github.com/roadrunner-server/api/v2/payload"
	"github.com/roadrunner-server/api/v2/plugins/config"
	"github.com/roadrunner-server/api/v2/plugins/server"
	"github.com/roadrunner-server/api/v2/pool"
	poolImpl "github.com/roadrunner-server/sdk/v2/pool"
	serverImpl "github.com/roadrunner-server/server/v2"
	"go.uber.org/zap"
)

type Foo4 struct {
	configProvider config.Configurer
	wf             server.Server
	pool           pool.Pool
	log            *zap.Logger
}

func (f *Foo4) Init(p config.Configurer, workerFactory server.Server, log *zap.Logger) error {
	f.configProvider = p
	f.wf = workerFactory
	f.log = log
	return nil
}

func (f *Foo4) Serve() chan error {
	// test payload for echo
	r := &payload.Payload{
		Context: []byte(`{"remoteAddr":"127.0.0.1","protocol":"HTTP/1.1","method":"GET","uri":"http://127.0.0.1:15389/","headers":{"Accept":["text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"],"Accept-Encoding":["gzip, deflate, br"],"Accept-Language":["en-GB,en-US;q=0.9,en;q=0.8,ru;q=0.7,pl;q=0.6,be;q=0.5"],"Cache-Control":["max-age=0"],"Connection":["keep-alive"],"Dnt":["1"],"Sec-Ch-Ua":["\".Not/A)Brand\";v=\"99\", \"Google Chrome\";v=\"103\", \"Chromium\";v=\"103\""],"Sec-Ch-Ua-Mobile":["?0"],"Sec-Ch-Ua-Platform":["\"Linux\""],"Sec-Fetch-Dest":["document"],"Sec-Fetch-Mode":["navigate"],"Sec-Fetch-Site":["none"],"Sec-Fetch-User":["?1"],"Upgrade-Insecure-Requests":["1"],"User-Agent":["Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36"]},"cookies":{},"rawQuery":"","parsed":false,"uploads":null,"attributes":{}}`),
		Body:    []byte(`{"foo":"bar"}`),
	}

	var testPoolConfig2 = &poolImpl.Config{
		NumWorkers:      1,
		AllocateTimeout: time.Second * 1000,
		DestroyTimeout:  time.Second * 1000,
	}

	errCh := make(chan error, 1)

	conf := &serverImpl.Config{}
	var err error
	err = f.configProvider.UnmarshalKey(ConfigSection, conf)
	if err != nil {
		errCh <- err
		return errCh
	}

	// test pool
	f.pool, err = f.wf.NewWorkerPool(context.Background(), testPoolConfig2, nil, f.log)
	if err != nil {
		errCh <- err
		return errCh
	}

	// test pool execution
	_, err = f.pool.Exec(r)
	if err != nil {
		errCh <- err
		return errCh
	}

	return errCh
}

func (f *Foo4) Stop() error {
	return nil
}

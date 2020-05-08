package internal

import (
	"expvar"

	"github.com/delving/hub3/ikuzo"
	"github.com/delving/hub3/ikuzo/service/x/ead"
)

type EAD struct {
	CacheDir string `json:"cacheDir"`
}

func (n *EAD) AddOptions(cfg *Config) error {
	is, err := cfg.GetIndexService()
	if err != nil {
		return err
	}

	svc, err := ead.NewService(
		ead.SetIndexService(is),
		ead.SetDataDir(n.CacheDir),
	)
	if err != nil {
		return err
	}

	expvar.Publish("hub3-ead-service", expvar.Func(func() interface{} { m := svc.Metrics(); return m }))

	cfg.options = append(
		cfg.options,
		ikuzo.SetEADService(svc),
		ikuzo.SetEnableLegacyConfig(),
	)

	return nil
}

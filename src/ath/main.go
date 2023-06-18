package ath

import (
	"errors"
	"fmt"

	"github.com/jessevdk/go-flags"
)

func Execute() error {
	config := Config{}
	if _, err := flags.Parse(&config); err != nil {
		if flags.WroteHelp(err) {
			return nil
		}
		return err
	}

	fmt.Printf("%+v", config)

	return errors.New("not yet implemented")
}

// package main is a module for featherwing OLED displays
package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"
	
	"github.com/biotinker/viam-i2c-display/display/api/displayapi"
	"github.com/biotinker/viam-i2c-display/display"
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewDevelopmentLogger("display"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	dispModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	dispModule.AddModelFromRegistry(ctx, displayapi.API, display.Model)
	
	err = dispModule.Start(ctx)
	defer dispModule.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

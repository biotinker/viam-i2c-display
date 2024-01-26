package display

import (
	"context"
	"encoding/hex"
	"math"
	"time"

	"github.com/biotinker/viam-i2c-display/display/api/displayapi"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

/*
		Values from the original arduino C library that I did not use, if you need them
	 * 	sh110xBLACK                   = 0    ///< Draw 'off' pixels
		sh110xWHITE                   = 1    ///< Draw 'on' pixels
		sh110xINVERSE                 = 2    ///< Invert pixels
		sh110xCOLUMNADDR         byte = 0x21 ///< See datasheet
		sh110xPAGEADDR           byte = 0x22 ///< See datasheet
		sh110xCHARGEPUMP         byte = 0x8D ///< See datasheet
		sh110xDISPLAYALLON       byte = 0xA5 ///< Not currently used
		sh110xINVERTDISPLAY      byte = 0xA7 ///< See datasheet
		sh110xDISPLAYON          byte = 0xAF ///< See datasheet
		sh110xSETPAGEADDR        byte = 0xB0 ///< Specify page address to load display RAM data to page address
		sh110xCOMSCANDEC         byte = 0xC8 ///< See datasheet
		sh110xSETCOMPINS         byte = 0xDA ///< See datasheet
		sh110xSETLOWCOLUMN       byte = 0x00 ///< Not currently used
		sh110xSETHIGHCOLUMN      byte = 0x10 ///< Not currently used
		sh110xSETSTARTLINE       byte = 0x40 ///< See datasheet
*/
const (
	sh110xMEMORYMODE         byte = 0x20 ///< See datasheet
	sh110xSETCONTRAST        byte = 0x81 ///< See datasheet
	sh110xSEGREMAP           byte = 0xA0 ///< See datasheet
	sh110xDISPLAYALLONRESUME byte = 0xA4 ///< See datasheet
	sh110xNORMALDISPLAY      byte = 0xA6 ///< See datasheet
	sh110xSETMULTIPLEX       byte = 0xA8 ///< See datasheet
	sh110xDCDC               byte = 0xAD ///< See datasheet
	sh110xDISPLAYOFF         byte = 0xAE ///< See datasheet
	sh110xCOMSCANINC         byte = 0xC0 ///< Not currently used
	sh110xSETDISPLAYOFFSET   byte = 0xD3 ///< See datasheet
	sh110xSETDISPLAYCLOCKDIV byte = 0xD5 ///< See datasheet
	sh110xSETPRECHARGE       byte = 0xD9 ///< See datasheet
	sh110xSETVCOMDETECT      byte = 0xDB ///< See datasheet
	sh110xSETDISPSTARTLINE   byte = 0xDC ///< Specify Column address to determine the initial display line or < COM0.
)

const defaultI2Caddr = 0x3C

var Model = resource.ModelNamespace("biotinker").WithFamily("component").WithModel("display")

// Config is used for converting config attributes.
type Config struct {
	I2CBus        string `json:"i2c_bus"`
	I2cAddr       int    `json:"i2c_addr,omitempty"`
	SkipAnimation bool   `json:"skip_animation",omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string
	if len(config.I2CBus) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		displayapi.API,
		Model,
		resource.Registration[displayapi.Display, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (displayapi.Display, error) {
				newConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				return newDisplay(ctx, deps, conf.ResourceName(), newConf, logger)
			},
		})
}

func newDisplay(
	ctx context.Context,
	deps resource.Dependencies,
	name resource.Name,
	attr *Config,
	logger logging.Logger,
) (*display, error) {
	i2cbus, err := buses.NewI2cBus(attr.I2CBus)
	if err != nil {
		return nil, err
	}
	addr := attr.I2cAddr
	if addr == 0 {
		addr = defaultI2Caddr
		logger.Warnf("using i2c address : 0x%s", hex.EncodeToString([]byte{byte(addr)}))
	}

	d := &display{
		Named:   name.AsNamed(),
		logger:  logger,
		bus:     i2cbus,
		addr:    byte(addr),
		current: blank(),
	}

	// Init the display multiple times, hoping at least one works- sometimes it takes several writes to get a good init
	for i := 0; i < 4; i++ {
		logger.Warn("init", i)
		d.initDisp(ctx)
	}

	if !attr.SkipAnimation {
		logger.Warn("animation")
		d.initAnimation(ctx)
	}

	return d, nil
}

func blank() []byte {
	return make([]byte, 1024)
}

// display is a i2c sensor device that reports voltage, current and power across N channels that should support multiple INA chip models
type display struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger  logging.Logger
	bus     buses.I2C
	addr    byte
	current []byte
}

func (d *display) DisplayBytes(ctx context.Context, data []byte) error {
	d.writeBuf(ctx, blank())
	new := make([]byte, len(d.current))
	for i, pix := range data {
		if i >= len(new) {
			break
		}
		new[i] = pix
	}
	return d.writeBuf(ctx, new)
}

func (d *display) WriteString(ctx context.Context, xloc, yloc int, text string) error {
	new := make([]byte, len(d.current))
	copy(new, d.current)

	new = writeString(xloc, yloc, text, new)
	return d.writeBuf(ctx, new)
}

func (d *display) DrawLine(ctx context.Context, x1, y1, x2, y2 int) error {
	new := make([]byte, len(d.current))
	copy(new, d.current)
	new = writeLine(x1, y1, x2, y2, new)
	return d.writeBuf(ctx, new)
}

func (d *display) Reset(ctx context.Context) error {
	d.initDisp(ctx)
	return d.writeBuf(ctx, blank())
}

func (d *display) initDisp(ctx context.Context) error {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)
	// set contrast
	contrast := []byte{0, 0x81, 0x2F}
	handle.Write(ctx, contrast)

	init := []byte{
		0x00,
		sh110xDISPLAYOFF,               // 0xAE
		sh110xSETDISPLAYCLOCKDIV, 0x51, // 0xd5, 0x51,
		sh110xMEMORYMODE,        // 0x20
		sh110xSETCONTRAST, 0x4F, // 0x81, 0x4F
		sh110xDCDC, 0x8A, // 0xAD, 0x8A
		sh110xSEGREMAP,              // 0xA0
		sh110xCOMSCANINC,            // 0xC0
		sh110xSETDISPSTARTLINE, 0x0, // 0xDC 0x00
		sh110xSETDISPLAYOFFSET, 0x60, // 0xd3, 0x60,
		sh110xSETPRECHARGE, 0x22, // 0xd9, 0x22,
		sh110xSETVCOMDETECT, 0x35, // 0xdb, 0x35,
		sh110xSETMULTIPLEX, 0x3F, // 0xa8, 0x3f,
		sh110xDISPLAYALLONRESUME, // 0xa4
		sh110xNORMALDISPLAY,      // 0xa6
	}

	handle.Write(ctx, init)

	time.Sleep(100 * time.Millisecond)

	// turn on
	handle.Write(ctx, []byte{0x00, 0xAF})
	return nil
}

func (d *display) checkInit(ctx context.Context) error {
	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		return err
	}
	buffer, _ := handle.Read(ctx, 1)
	err = handle.Close()
	if err != nil {
		return err
	}
	if buffer[0] == 71 {
		d.initDisp(ctx)
	}
	return nil
}

func (d *display) initAnimation(ctx context.Context) {
	buf := blank()
	for i := 1; i < 15; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		buf = writeFillRect(i*8, 20, 8, 24, buf)
		d.writeBuf(ctx, buf)
	}
	d.writeBuf(ctx, blank())
}

// This actually writes the buffered bytes to the display
func (d *display) writeBuf(ctx context.Context, buf []byte) error {

	d.checkInit(ctx)

	handle, err := d.bus.OpenHandle(d.addr)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	var reg byte
	iter := 0
	for reg = 0xB0; reg <= 0xBF; reg++ {
		someBytes := []byte{0, reg, 0x10, 0}
		handle.Write(context.Background(), someBytes)

		someBytes = append([]byte{0x40}, buf[0+iter*64:31+iter*64]...)
		handle.Write(context.Background(), someBytes)
		someBytes = append([]byte{0x40}, buf[31+iter*64:62+iter*64]...)
		handle.Write(context.Background(), someBytes)

		someBytes = []byte{0x40, buf[62+iter*64], buf[63+iter*64]}
		handle.Write(context.Background(), someBytes)

		iter++
	}
	d.current = buf
	return nil
}

func writePixel(x, y int, buf []byte) []byte {
	x, y = y, x

	WIDTH := 64
	LENGTH := 128
	for x >= WIDTH {
		x -= WIDTH
	}
	for x < 0 {
		x += WIDTH
	}
	for y >= LENGTH {
		y -= LENGTH
	}
	for y < 0 {
		y += LENGTH
	}

	idx := x + (y/8)*WIDTH
	blen := (WIDTH * LENGTH) / 8
	for idx >= blen {
		idx -= blen
	}

	buf[idx] |= (1 << (y & 7))
	return buf
}

// Write a line.  Bresenham's algorithm
func writeLine(x0, y0, x1, y1 int, buf []byte) []byte {
	steep := math.Abs(float64(y1-y0)) > math.Abs(float64(x1-x0))
	if steep {
		x0, y0 = y0, x0
		x1, y1 = y1, x1
	}

	if x0 > x1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
	}

	dx := x1 - x0
	dy := y1 - y0
	if dy < 0 {
		dy *= -1
	}

	err := dx / 2
	ystep := -1

	if y0 < y1 {
		ystep = 1
	}

	for x0 <= x1 {
		if steep {
			buf = writePixel(y0, x0, buf)
		} else {
			buf = writePixel(x0, y0, buf)
		}
		err -= dy
		if err < 0 {
			y0 += ystep
			err += dx
		}
		x0++
	}
	return buf
}

func writeFillRect(x, y, w, h int, buf []byte) []byte {
	for i := x; i < x+w; i++ {
		buf = writeLine(i, y, i, y+h, buf)
	}
	return buf
}

func writeString(x, y int, char string, buf []byte) []byte {

	charBytes := []byte(char)

	for _, cb := range charBytes {
		charIdx := cb - 0x20
		if cb < 0x20 || charIdx >= 95 {
			continue
		}
		cInfo := chars[charIdx]
		// byte offset
		bo := cInfo[0]
		w := cInfo[1]
		h := cInfo[2]
		adv := cInfo[3]
		xo := cInfo[4]
		yo := cInfo[5]

		var bit byte
		var bits byte

		for yy := 0; yy < h; yy++ {
			for xx := 0; xx < w; xx++ {
				if bit&7 == 0 {
					bits = freemono[bo]
					bo++
				}
				bit++
				if (bits & 0x80) > 0 {
					//~ buf = writePixel(x+xo+xx, y+yo+(h-yy), buf)
					buf = writePixel(x+xo+xx, (y-yo)-yy, buf)
				}
				bits <<= 1
			}
		}
		x += adv
	}
	return buf
}

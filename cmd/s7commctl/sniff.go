package s7commctl

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/spf13/cobra"
	"otfabric/s7comm/wire"
)

type sniffConfig struct {
	Listen   string `flag:"listen" env:"S7COMMCTL_SNIFF_LISTEN" desc:"Listen address for passive frame feed"`
	Format   string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, jsonl"`
	MaxFrame int    `flag:"max-frames" env:"S7COMMCTL_MAX_FRAMES" desc:"Stop after this many frames (0 = unlimited)"`
}

var snfConfig = sniffConfig{
	Listen:   ":15102",
	Format:   "text",
	MaxFrame: 0,
}

var sniffCmd = &cobra.Command{
	Use:   "sniff",
	Short: "Passive S7 frame decoder over TCP stream input",
	Long:  "Listens for raw TPKT frames on a local TCP socket and decodes them without initiating any PLC communication.",
	RunE:  runSniff,
}

func init() {
	config.RegisterFlags(sniffCmd, &snfConfig)
	config.LoadFromEnv(&snfConfig)
	rootCmd.AddCommand(sniffCmd)
}

func runSniff(cmd *cobra.Command, args []string) error {
	ln, err := net.Listen("tcp", snfConfig.Listen)
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	defer func() { _ = ln.Close() }()

	_, _ = fmt.Fprintf(os.Stdout, "sniff listening on %s\n", snfConfig.Listen)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accept failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	frames := 0
	for snfConfig.MaxFrame == 0 || frames < snfConfig.MaxFrame {
		if err := conn.SetReadDeadline(time.Now().Add(120 * time.Second)); err != nil {
			return err
		}
		frame, rerr := readTPKTFrame(conn)
		if rerr != nil {
			_ = writeFrameEvent(os.Stdout, snfConfig.Format, conn.RemoteAddr().String(), nil, rerr)
			return nil
		}

		summary, derr := wire.InspectFrame(frame)
		if err := writeFrameEvent(os.Stdout, snfConfig.Format, conn.RemoteAddr().String(), summary, derr); err != nil {
			return err
		}
		frames++
	}

	return nil
}

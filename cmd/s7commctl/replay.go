package s7commctl

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/otfabric/s7comm/wire"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/spf13/cobra"
)

type replayConfig struct {
	Listen     string `flag:"listen" env:"S7COMMCTL_REPLAY_LISTEN" desc:"Replay listen address"`
	ScriptFile string `flag:"script" env:"S7COMMCTL_REPLAY_SCRIPT" desc:"Script file with hex-encoded response frames"`
	Format     string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, jsonl"`
	DelayMS    int    `flag:"delay-ms" env:"S7COMMCTL_REPLAY_DELAY_MS" desc:"Delay before each scripted response"`
}

var rplConfig = replayConfig{
	Listen:  ":1102",
	Format:  "text",
	DelayMS: 0,
}

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replay scaffold for deterministic S7 response scripts",
	Long:  "Accepts incoming frames and optionally responds with scripted raw TPKT hex frames, one response per request. Single-client mode: accepts one TCP connection; processes one request/response pair per scripted line until the connection closes.",
	RunE:  runReplay,
}

func init() {
	config.RegisterFlags(replayCmd, &rplConfig)
	config.LoadFromEnv(&rplConfig)
	rootCmd.AddCommand(replayCmd)
}

func runReplay(cmd *cobra.Command, args []string) error {
	responses, err := loadReplayScript(rplConfig.ScriptFile)
	if err != nil {
		return ExitUsage(err)
	}

	ln, err := net.Listen("tcp", rplConfig.Listen)
	if err != nil {
		return ExitRuntime(fmt.Errorf("listen failed: %w", err))
	}
	defer func() { _ = ln.Close() }()

	_, _ = fmt.Fprintf(os.Stdout, "replay listening on %s\n", rplConfig.Listen)
	conn, err := ln.Accept()
	if err != nil {
		return ExitRuntime(fmt.Errorf("accept failed: %w", err))
	}
	defer func() { _ = conn.Close() }()

	idx := 0
	for {
		if err := conn.SetReadDeadline(time.Now().Add(300 * time.Second)); err != nil {
			return ExitRuntime(err)
		}
		frame, rerr := readTPKTFrame(conn)
		if rerr != nil {
			_ = writeFrameEvent(os.Stdout, rplConfig.Format, conn.RemoteAddr().String(), nil, rerr)
			if errors.Is(rerr, io.EOF) || errors.Is(rerr, net.ErrClosed) {
				return nil
			}
			return ExitRuntime(rerr)
		}

		summary, derr := wire.InspectFrame(frame)
		if err := writeFrameEvent(os.Stdout, rplConfig.Format, conn.RemoteAddr().String(), summary, derr); err != nil {
			return ExitRuntime(err)
		}

		if idx >= len(responses) {
			continue
		}
		if rplConfig.DelayMS > 0 {
			time.Sleep(time.Duration(rplConfig.DelayMS) * time.Millisecond)
		}
		if _, err := conn.Write(responses[idx]); err != nil {
			return ExitRuntime(fmt.Errorf("write scripted response: %w", err))
		}
		idx++
	}
}

func loadReplayScript(path string) ([][]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open replay script: %w", err)
	}
	defer func() { _ = f.Close() }()

	var responses [][]byte
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ReplaceAll(line, " ", "")
		b, err := hex.DecodeString(line)
		if err != nil {
			return nil, fmt.Errorf("invalid replay script line %q: %w", s.Text(), err)
		}
		if len(b) < 4 || b[0] != 0x03 {
			return nil, fmt.Errorf("script line is not a TPKT frame: %q", s.Text())
		}
		responses = append(responses, b)
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read replay script: %w", err)
	}

	return responses, nil
}

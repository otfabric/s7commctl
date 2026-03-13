package s7commctl

import (
	"fmt"
	"os"

	"github.com/otfabric/s7commctl/internal/app/validate"
)

func warnIfPublicTarget(ip string, quiet bool, detail string, noExposureWarning bool) {
	if quiet || noExposureWarning {
		return
	}
	if validate.IsPublicIP(ip) {
		if detail != "" {
			_, _ = fmt.Fprintf(os.Stderr, "WARNING: target appears publicly routable; %s\n", detail)
			return
		}
		_, _ = fmt.Fprintln(os.Stderr, "WARNING: target appears publicly routable; verify authorization and network exposure")
	}
}

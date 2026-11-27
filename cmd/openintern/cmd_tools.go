package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dnexdev/openintern/internal/tools"
)

func init() {
	register("gap-report", "diff the SpeedyApply/SimplifyJobs lists against data/companies (--ats probes Apply-link boards)", runGapReport)
	register("discover-accelerators", "find public ATS boards for YC / a16z portfolio companies (--yc, --a16z)", runDiscoverAccelerators)
	register("recover-tokens", "guess working board tokens for inactive or broken companies", runRecoverTokens)
	register("validate-tokens", "probe the boards of company YAML files (CI: only boards a PR adds or edits)", runValidateTokens)
}

const toolsDataUsage = "  -data dir   registry directory (default data, in the working directory)\n"

// toolArgs takes the -data flag out of args and leaves the rest in the
// order the scripts read them (they matched flags anywhere, ignored unknown
// ones and took file names from whatever did not start with "--"). help is
// true for -h/--help.
func toolArgs(args []string) (dataDir string, rest []string, help bool, err error) {
	dataDir = "data"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "-help" || a == "--help":
			help = true
		case a == "-data" || a == "--data":
			if i+1 >= len(args) {
				return "", nil, false, fmt.Errorf("%s needs a directory", a)
			}
			dataDir = args[i+1]
			i++
		case strings.HasPrefix(a, "-data=") || strings.HasPrefix(a, "--data="):
			dataDir = a[strings.IndexByte(a, '=')+1:]
		default:
			rest = append(rest, a)
		}
	}
	return dataDir, rest, help, nil
}

func runGapReport(args []string) error {
	dataDir, rest, help, err := toolArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stderr, tools.GapReportUsage+toolsDataUsage)
		return nil
	}
	return tools.GapReport(context.Background(), tools.DefaultEnv(dataDir), rest)
}

func runDiscoverAccelerators(args []string) error {
	dataDir, rest, help, err := toolArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stderr, "usage: openintern discover-accelerators --yc | --a16z [--write] [--limit=N]\n"+toolsDataUsage)
		return nil
	}
	code, err := tools.DiscoverAccelerators(context.Background(), tools.DefaultEnv(dataDir), rest)
	if err != nil {
		return err
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func runRecoverTokens(args []string) error {
	dataDir, rest, help, err := toolArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stderr, "usage: openintern recover-tokens [--limit=N] [--slug=a,b] [--write]\n"+toolsDataUsage)
		return nil
	}
	return tools.RecoverTokens(context.Background(), tools.DefaultEnv(dataDir), rest)
}

func runValidateTokens(args []string) error {
	dataDir, rest, help, err := toolArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(os.Stderr, "usage: openintern validate-tokens [--all] [file.yaml ...]\n"+
			"  with GITHUB_BASE_REF set, only boards added or edited since origin/$GITHUB_BASE_REF are probed;\n"+
			"  with CI=true, dump-gated proprietary boards that refuse the runner are skipped\n"+toolsDataUsage)
		return nil
	}
	code, err := tools.ValidateTokens(context.Background(), tools.DefaultEnv(dataDir), rest)
	if err != nil {
		return err
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

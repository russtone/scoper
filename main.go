package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/russtone/iprange"
	"github.com/russtone/ipsearch"
)

var (
	ranges    []string // slice of IP ranges from "-r" args
	rangeFile string   // path to files with ranges from "-R" arg
	colors    bool     // color mode, "-c" arg

	scope  iprange.Ranges // all ranges combined
	inputs []*os.File     // all inputs combined
)

func init() {
	cmd.PersistentFlags().StringArrayVarP(&ranges, "range", "r", []string{}, "IP range.\n"+
		"Examples: \n"+
		"- 192.168.1.1\n"+
		"- 192.168.1.0/22\n"+
		"- 192.168.1.0_192.168.2.254\n"+
		"- 192.168.1-2,5.0-255\n"+
		"- fe80::1:2:3:4\n"+
		"- fe80::1:2:3:4/64\n"+
		"- fe80::1:2:3:4_fe80::1:2:3:50\n"+
		"- fe80::1:2:3:1,2,4-a",
	)
	cmd.PersistentFlags().StringVarP(&rangeFile, "range-file", "R", "", "path to file with IP ranges on each line")
	cmd.PersistentFlags().BoolVarP(&colors, "color", "c", false, "color mode: print all lines,\n"+
		"but highlight in scope IP addresses with green and rest with red")
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
}

// cmd represents the base command when called without any subcommands.
var cmd = &cobra.Command{
	Use:           "scoper",
	Short:         "Filters lines containing IP addresses from scope",
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.ArbitraryArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		scope = make(iprange.Ranges, 0)

		// IP ranges from command line arguments.
		for _, s := range ranges {
			r := iprange.Parse(s)
			if r == nil {
				return fmt.Errorf("invalid IP range: %q", s)
			}
			scope = append(scope, r)
		}

		// IP ranges from file.
		if rangeFile != "" {
			if _, err := os.Stat(rangeFile); os.IsNotExist(err) {
				return fmt.Errorf("no such file: %q", rangeFile)
			}

			rf, err := os.Open(rangeFile)
			if err != nil {
				return fmt.Errorf("fail to open file: %q", rangeFile)
			}
			defer rf.Close()

			scanner := bufio.NewScanner(rf)

			for scanner.Scan() {
				s := scanner.Text()
				r := iprange.Parse(scanner.Text())
				if r == nil {
					return fmt.Errorf("invalid IP range: %q", s)
				}
				scope = append(scope, r)
			}

			if err := scanner.Err(); err != nil {
				return err
			}
		}

		if len(scope) == 0 {
			return fmt.Errorf("empty scope")
		}

		// Check if there is some data in the stdin.
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			inputs = append(inputs, os.Stdin)
		}

		for _, fpath := range args {
			if _, err := os.Stat(fpath); os.IsNotExist(err) {
				return fmt.Errorf("no such file: %q", fpath)
			}

			file, err := os.Open(fpath)
			if err != nil {
				return fmt.Errorf("fail to open file: %q", fpath)
			}

			inputs = append(inputs, file)
		}

		if len(inputs) == 0 {
			return fmt.Errorf("no inputs")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		green := color.New(color.FgGreen).Add(color.Bold)
		red := color.New(color.FgRed).Add(color.Bold)

		// Loop through all inputs.
		for _, input := range inputs {
			scanner := bufio.NewScanner(input)

			// Loop through input lines.
			for scanner.Scan() {
				line := scanner.Text()
				show := false

				// Loop through all IP addresses in line.
				for _, r := range ipsearch.Find(line) {
					ip := net.ParseIP(r)
					if ip == nil {
						continue
					}

					if colors {
						// In colors mode highlight IP in scope with green color
						// and rest with red.
						var colorized string

						if scope.Contains(ip) {
							colorized = green.Sprintf("%s", ip)
						} else {
							colorized = red.Sprintf("%s", ip)
						}

						// Replace IP in line with colorized IP.
						line = strings.Replace(line, ip.String(), colorized, -1)

					} else {
						// In normal mode print only lines containing IP from scope.
						if scope.Contains(ip) {
							show = true
							break
						}
					}
				}

				// Print line if color mode or line contains IP from scope.
				if colors || show {
					fmt.Println(line)
				}
			}

			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %s\n", err)
			}
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		for _, input := range inputs {
			if input != os.Stdin {
				input.Close()
			}
		}
	},
}

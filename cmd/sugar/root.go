package sugar

import (
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type rootCmd struct {
	cmd   *cobra.Command
	debug bool
	exit  func(int)

	loggerInstance *zerolog.Logger
}

func newRootCmd(exit func(int), redisConn *redis.Client, loggerInstance *zerolog.Logger) *rootCmd {
	root := &rootCmd{
		exit:           exit,
		loggerInstance: loggerInstance,
	}

	cmd := &cobra.Command{
		Use:           "rvc",
		Short:         "random video chat",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
		},
	}

	cmd.PersistentFlags().BoolVar(&root.debug, "debug", false, "Enable debug mode")
	cmd.AddCommand(
		newUserCmd(redisConn, loggerInstance).cmd,
		newUserEventCmd(redisConn, loggerInstance).cmd,
		newUserConnectionCmd(redisConn, loggerInstance).cmd,
		newForwarderCmd(redisConn, loggerInstance).cmd,
	)

	root.cmd = cmd

	return root
}

func (cmd *rootCmd) Execute(args []string) {
	cmd.cmd.SetArgs(commander(cmd.cmd, args))

	err := cmd.cmd.Execute()
	if err != nil {
		cmd.loggerInstance.Err(err).Msg("failed to execute command")
		cmd.exit(1) // exits with code 1, i.e. general error
	}
}

func commander(cmd *cobra.Command, args []string) []string {
	set := map[string]bool{
		"-h":        true,
		"--help":    true,
		"--version": true,
		"help":      true,
	}

	xmd, _, _ := cmd.Find(args)

	if xmd != nil {
		if len(args) > 1 && args[1] == "help" {
			args[1] = "--help"
		}
		return args
	}

	if len(args) > 0 &&
		(args[0] == "completion" ||
			args[0] == cobra.ShellCompRequestCmd ||
			args[0] == cobra.ShellCompNoDescRequestCmd) {
		return args
	}

	if len(args) == 0 || (len(args) == 1 && set[args[0]]) {
		return args
	}

	return []string{"help"}
}

func Execute(exit func(int), args []string, redisConn *redis.Client, loggerInstance *zerolog.Logger) {
	newRootCmd(exit, redisConn, loggerInstance).Execute(args)
}

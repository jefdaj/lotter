Package command provides consistency in parsing command line flags, reading config files, and writing logs. This package is intended for launching commands with operations (sub-commands).

Commands take the form:

command [flag ...] <operation> [operation flag ...] [arg ...]

Common Operations

Provides a "help" psuedo-operation, which may be invoked as:

command help              # shows general usage
command help <operation>  # shows operation-specific usage

Common Flags

Top level flags include "-v" for verbosity, and "-config" to specify the directory where configuration files are found.
Flag Helpers

Flags support provided by the golang standard flags package. The command package defines several types to support more advanced flag features.

See StringSet, BoolMap, and BoolCount.
Log Helpers

Logging API is a simple addition to Go stdlib log package.

The V() helper limits verbosity. For example,

command.V(2).Log("hello world")

will write only if the "-v" flag appears twice (or more) in the command flags.

If command.Error() or command.Errorf() is called, then command.Exit() will terminate with a non-zero status.


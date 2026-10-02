import { Command, Option } from "commander";
import { CliError } from "./context.js";
import { VERSION } from "./version.js";

// 2.0.0 removed the 1.x choices of the legacy knowledge model. They still parse, so a 1.x
// invocation fails before any request, naming what replaces it, instead of failing as an
// unknown command or running without the choice it asked for.

/** The error for a removed 1.x command, argument or option. */
export function removedError(name: string, instead: string): CliError {
  return new CliError(
    `'${name}' was removed in @solvr/cli ${VERSION}; ${instead}. See "Migrating from 1.x to ${VERSION}" in the README.`
  );
}

/** Keeps a removed command out of --help; running it, with any arguments, fails. */
export function removedCommand(program: Command, name: string, instead: string): void {
  program
    .command(name, { hidden: true })
    .allowUnknownOption()
    .allowExcessArguments()
    .action(() => {
      throw removedError(`solvr ${name}`, instead);
    });
}

/** Keeps a removed option of cmd out of --help; giving it fails before the command runs. */
export function removedOption(cmd: Command, flags: string, instead: string): Command {
  const option = new Option(flags).hideHelp();
  cmd.addOption(option);
  return cmd.hook("preAction", (command) => {
    if (command.getOptionValue(option.attributeName()) !== undefined) {
      throw removedError(option.long ?? flags, instead);
    }
  });
}

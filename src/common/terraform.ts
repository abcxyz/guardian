import { EOL } from 'os';
import { getExecOutput, ExecOutput } from '@actions/exec';

export interface TerraformClient {
  formatGitHubDiff(text: string): string;
  format(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  init(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  validate(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  plan(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  apply(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  show(args?: string[], workingDirectory?: string): Promise<ExecOutput>;
  command(workingDirectory: string, command: string, args?: string[]): Promise<ExecOutput>;
}

export class ActionsTerraformClient implements TerraformClient {
  readonly #workingDirectory: string;

  constructor(workingDirectory: string, automation: boolean = false) {
    this.#workingDirectory = workingDirectory;

    if (automation) {
      process.env.TF_IN_AUTOMATION = 'true';
    }
  }

  formatGitHubDiff(text: string): string {
    return text.replace(/^(\s+)(\~)/gm, '$1!').replace(/^(\s+)([\!\-\+])/gm, '$2$1');
  }

  async format(args: string[], workingDirectory?: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['fmt', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async init(args: string[] = [], workingDirectory: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['init', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async validate(args: string[] = [], workingDirectory: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['validate', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async plan(args: string[] = [], workingDirectory: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['plan', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
      ignoreReturnCode: true,
    });

    if (output.exitCode === 1) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async apply(args: string[] = [], workingDirectory: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['apply', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async show(args: string[] = [], workingDirectory: string): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', ['show', ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }

  async command(
    workingDirectory: string,
    command: string,
    args: string[] = [],
  ): Promise<ExecOutput> {
    const output = await getExecOutput('terraform', [command, ...args], {
      cwd: workingDirectory || this.#workingDirectory,
    });

    if (output.exitCode !== 0) {
      throw new Error(`${output.stdout}${EOL}${output.stderr}`);
    }

    return output;
  }
}

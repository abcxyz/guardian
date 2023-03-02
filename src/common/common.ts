import { Context } from '@actions/github/lib/context';

export interface Config {
  envs: EnvironmentVariables;
  inputs: Inputs;
  context: Context;
}

export interface EnvironmentVariables {
  repositoryId: string;
  githubToken: string;
  runAttempt: string;
}

export interface Inputs {
  workingDirectory: string;
  bucketName: string;
  maxRetries: number;
  baseRetryDelay: number | undefined;
  maxRetryDelay: number | undefined;
}

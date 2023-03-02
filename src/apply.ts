/*
 * Copyright 2020 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import fs from 'fs';

import {
  getInput,
  setFailed,
  info as logInfo,
  warning as logWarning,
  error as logError,
} from '@actions/core';

import * as github from '@actions/github';

import { errorMessage } from '@google-github-actions/actions-utils';

import { Config } from './common/common';
import { ActionsGitHubClient, GitHubClient } from './common/github';
import { ActionsStorageClient, StorageClient } from './common/storage';
import { ActionsTerraformClient, TerraformClient } from './common/terraform';

/**
 * Run is the primary entrypoint and is a wrapper function
 * to create and provide dependencies to the main function.
 */
export async function run(): Promise<void> {
  try {
    // Get action environment variables
    const {
      GITHUB_REPOSITORY_ID: repositoryId,
      GITHUB_TOKEN: githubToken,
      GITHUB_RUN_ATTEMPT: runAttempt,
    } = process.env;

    if (!githubToken) {
      throw new Error('environemnt variable GITHUB_TOKEN is required to use this action.');
    }

    // Get action inputs
    const workingDirectory = getInput('working_directory');
    const bucketName = getInput('bucket_name');
    const maxRetries = Number(getInput('max_retries'));
    const baseRetryDelay = Number(getInput('base_retry_delay'));
    const maxRetryDelay = Number(getInput('max_retry_delay'));

    const config: Config = {
      envs: {
        repositoryId: repositoryId || '',
        githubToken: githubToken,
        runAttempt: runAttempt || '',
      },
      inputs: {
        workingDirectory: workingDirectory,
        bucketName: bucketName,
        maxRetries: maxRetries,
        baseRetryDelay: baseRetryDelay,
        maxRetryDelay: maxRetryDelay,
      },
      context: github.context,
    };

    main(
      config,
      new ActionsGitHubClient(githubToken, {
        retry: {
          retries: config.inputs.maxRetries,
          backoff: config.inputs.baseRetryDelay,
          backoffLimit: config.inputs.maxRetryDelay,
        },
      }),
      new ActionsStorageClient({
        retryOptions: {
          autoRetry: true,
          maxRetries: config.inputs.maxRetries,
          maxRetryDelay: config.inputs.maxRetryDelay,
        },
      }),
      new ActionsTerraformClient(config.inputs.workingDirectory, true),
    );
  } catch (err) {
    const msg = errorMessage(err);
    setFailed(`Guardian Apply failed to initialize: ${msg}`);
  }
}

/**
 * Executes the main action. It includes the main business logic and is the
 * primary entry point. It is documented inline.
 */
export async function main(
  config: Config,
  githubClient: GitHubClient,
  storageClient: StorageClient,
  terraformClient: TerraformClient,
): Promise<void> {
  const workflowHTMLURL = `${config.context.serverUrl}/${config.context.repo.owner}/${config.context.repo.repo}/actions/runs/${config.context.runId}/attempts/${config.envs.runAttempt}`;

  const lockPrefix = `guardian-locks/${config.context.repo.owner}/${config.context.repo.repo}`;
  const lockFilename = `${config.envs.repositoryId}.tflock`;

  const planRemovePrefix = `guardian-plans/${config.context.repo.owner}/${config.context.repo.repo}/${config.context.issue.number}`;
  const planPrefix = `guardian-plans/${config.context.repo.owner}/${config.context.repo.repo}/${config.context.issue.number}/${config.inputs.workingDirectory}`;
  const planFilename = `${config.context.sha}.tfplan`;

  try {
    fs.writeFileSync(lockFilename, '', { encoding: 'utf8' });

    await storageClient.validateLock(
      config.inputs.bucketName,
      `${lockPrefix}/${lockFilename}`,
      String(config.context.issue.number),
      true,
    );

    const planFileMetadata = await storageClient.getMetadata(
      config.inputs.bucketName,
      `${planPrefix}/${planFilename}`,
      false,
    );

    if (!Boolean(planFileMetadata.metadata.hasDiff)) {
      process.exit(0);
    }

    await storageClient.downloadFile(
      config.inputs.bucketName,
      `${planPrefix}/${planFilename}`,
      `${config.inputs.workingDirectory}/${planFilename}`,
    );

    await terraformClient.init(['-input=false', '-no-color']);

    const apply = await terraformClient.apply(['-input=false', '-no-color', planFilename]);
    const commentOutput = terraformClient.formatGitHubDiff(apply.stdout);

    await storageClient.deleteFiles(config.inputs.bucketName, planRemovePrefix);

    await githubClient.createPRComment(
      config.context.repo.owner,
      config.context.repo.repo,
      config.context.issue.number,
      `**\`🔱 Guardian 🔱\`** - 🟩 Ran Apply in dir: \`${config.inputs.workingDirectory}\` [[logs](${workflowHTMLURL})]\n
<details>
<summary>Diff</summary>\n
\`\`\`diff\n
${commentOutput}
\`\`\`
</details>`,
    );
  } catch (err) {
    const msg = errorMessage(err) || `Failed to run Guardian Apply.`;

    await githubClient.createPRComment(
      config.context.repo.owner,
      config.context.repo.repo,
      config.context.issue.number,
      `**\`🔱 Guardian 🔱\`** - 🟥 Failed to run Apply in dir: \`${config.inputs.workingDirectory}\` [[logs](${workflowHTMLURL})]\n
<details>
<summary>Details</summary>\n
\`\`\`diff\n
${msg}
\`\`\`
</details>`,
    );

    setFailed(`Guardian Apply failed with: ${msg}`);
  } finally {
    await storageClient.removeLock(
      config.inputs.bucketName,
      `${lockPrefix}/${lockFilename}`,
      String(config.context.issue.number),
    );
  }
}

/**
 * Executes the main function when this module is required directly.
 */
if (require.main === module) {
  run();
}

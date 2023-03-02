/*
 * Copyright 2023 Google LLC
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

import 'mocha';
import { expect } from 'chai';
import * as sinon from 'sinon';

import { errorMessage } from '@google-github-actions/actions-utils';

import { main as plan } from '../src/plan';
import { main as apply } from '../src/apply';

import { Config } from '../src/common/common';
import { GitHubClient } from '../src/common/github';

import { MockGitHubClient } from './mocks/github.test';
import { MockStorageClient } from './mocks/storage.test';
import { ActionsStorageClient } from '../src/common/storage';
import { ActionsTerraformClient } from '../src/common/terraform';

describe('#plan', function () {
  beforeEach(async function () {
    this.stubs = {};

    // sinon.stub(core, 'endGroup').callsFake(sinon.fake());
    // sinon.stub(core, 'startGroup').callsFake(sinon.fake());
    // sinon.stub(core, 'debug').callsFake(sinon.fake());
    // sinon.stub(core, 'info').callsFake(sinon.fake());
    // sinon.stub(core, 'error').callsFake(sinon.fake());
    // sinon.stub(core, 'warning').callsFake(sinon.fake());
  });

  afterEach(async function () {
    Object.keys(this.stubs).forEach((k) => this.stubs[k].restore());
    sinon.restore();
  });

  it('executes main successfully', async function () {
    const config: Config = generateConfig(1);

    const mockGitHubClient: GitHubClient = new MockGitHubClient();
    //const mockStorageClient: StorageClient = new MockStorageClient();
    const storageClient = new ActionsStorageClient({
      retryOptions: {
        autoRetry: true,
        maxRetries: 2,
        maxRetryDelay: 20,
      },
    });
    const terraformClient = new ActionsTerraformClient(config.inputs.workingDirectory, true);

    await plan(config, mockGitHubClient, storageClient, terraformClient);
    await apply(config, mockGitHubClient, storageClient, terraformClient);
  });
});

function generateConfig(number: number): Config {
  return {
    envs: {
      repositoryId: '11111111',
      githubToken: 'GITHUB_TOKEN',
      runAttempt: '1',
    },
    inputs: {
      workingDirectory: 'test/terraform',
      bucketName: 'verbanicm-dev-terraform',
      maxRetries: 5,
      baseRetryDelay: 2,
      maxRetryDelay: 60,
    },
    context: {
      payload: {
        pull_request: {
          number: number,
          html_url: 'https://github.com/abcxyz/guardian',
        },
      },
      eventName: '',
      sha: 'GIT_SHA',
      ref: '',
      workflow: '',
      action: '',
      actor: 'guardian-unit-test',
      job: '1',
      runNumber: 1,
      runId: 1,
      apiUrl: '',
      serverUrl: '',
      graphqlUrl: '',
      issue: {
        owner: 'owner',
        repo: 'repo',
        number: number,
      },
      repo: {
        owner: 'owner',
        repo: 'repo',
      },
    },
  };
}

async function expectError(fn: () => Promise<void>, want: string) {
  try {
    await fn();
    throw new Error(`expected error`);
  } catch (err) {
    const msg = errorMessage(err);
    expect(msg).to.include(want);
  }
}

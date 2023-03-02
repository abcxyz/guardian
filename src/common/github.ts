import * as github from '@actions/github';
import { RetryOptions, withRetries } from '@google-github-actions/actions-utils/dist';
import { Octokit } from '@octokit/core';
import { PaginateInterface } from '@octokit/plugin-paginate-rest';
import { Api } from '@octokit/plugin-rest-endpoint-methods/dist-types/types';

export interface GitHubClient {
  createPRComment(owner: string, repo: string, number: number, body: string): Promise<number>;
  updatePRComment(owner: string, repo: string, id: number, body: string): Promise<void>;
}

export interface GitHubClientOptions {
  retry: RetryOptions;
}

export class ActionsGitHubClient implements GitHubClient {
  readonly #client;
  readonly #options: GitHubClientOptions;

  constructor(token: string, options: GitHubClientOptions) {
    this.#client = github.getOctokit(token);
    this.#options = options;
  }

  async createPRComment(
    owner: string,
    repo: string,
    number: number,
    body: string,
  ): Promise<number> {
    const createPRCommentWithRetries = await withRetries(
      async () => {
        return await this.#client.rest.issues.createComment({
          owner: owner,
          repo: repo,
          issue_number: number,
          body: body,
        });
      },
      { ...this.#options.retry },
    );
    const { data: prComment } = await createPRCommentWithRetries();
    return prComment.id;
  }

  async updatePRComment(owner: string, repo: string, number: number, body: string): Promise<void> {
    const updatePRCommentWithRetries = await withRetries(
      async () => {
        await this.#client.rest.issues.updateComment({
          owner: owner,
          repo: repo,
          comment_id: number,
          body: body,
        });
      },
      { ...this.#options.retry },
    );
    return updatePRCommentWithRetries();
  }
}

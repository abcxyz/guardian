import { GitHubClient } from '../../src/common/github';

export class MockGitHubClient implements GitHubClient {
  constructor() {}
  async createPRComment(
    owner: string,
    repo: string,
    number: number,
    body: string,
  ): Promise<number> {
    return Promise.resolve(1);
  }
  async updatePRComment(owner: string, repo: string, id: number, body: string): Promise<void> {
    return Promise.resolve();
  }
}

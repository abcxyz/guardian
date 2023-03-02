import { Storage, StorageOptions } from '@google-cloud/storage';
import { ApiError } from '@google-cloud/storage/build/src/nodejs-common';

export interface StorageClient {
  uploadFile(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    metadata: { [key: string]: string },
  ): Promise<void>;
  downloadFile(bucket: string, remoteFilename: string, destFilePath: string): Promise<void>;
  deleteFiles(bucket: string, prefix: string): Promise<void>;
  deleteFile(bucket: string, filename: string): Promise<void>;
  getMetadata(bucket: string, filename: string, ignoreNotFound?: boolean): Promise<any>;
  createLock(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    lockMetadata: LockMetadata,
  ): Promise<void>;
  validateLock(
    bucket: string,
    filename: string,
    lockId: string,
    expectLockfile: boolean,
  ): Promise<void>;
  removeLock(bucket: string, filename: string, lockId: string): Promise<void>;
}

export interface LockMetadata {
  lockId: string;
  lockRepoURL: string;
}

export class ActionsStorageClient implements StorageClient {
  readonly #client;

  constructor(options: StorageOptions) {
    this.#client = new Storage(options);
  }

  async uploadFile(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    metadata: { [key: string]: string },
  ): Promise<void> {
    await this.#client.bucket(bucket).upload(localFilePath, {
      destination: destFilename,
      metadata: {
        metadata: metadata,
      },
    });
  }

  async downloadFile(bucket: string, remoteFileName: string, destFilePath: string): Promise<void> {
    await this.#client.bucket(bucket).file(remoteFileName).download({ destination: destFilePath });
  }

  async deleteFiles(bucket: string, prefix: string): Promise<void> {
    await this.#client.bucket(bucket).deleteFiles({ prefix: prefix, versions: true });
  }

  async deleteFile(bucket: string, filename: string): Promise<void> {
    await this.#client.bucket(bucket).file(filename).delete();
  }

  async getMetadata(
    bucket: string,
    filename: string,
    ignoreNotFound: boolean = false,
  ): Promise<any> {
    try {
      const [metadata] = await this.#client.bucket(bucket).file(filename).getMetadata();
      return metadata;
    } catch (err) {
      const apiError = err as ApiError;

      if (ignoreNotFound && apiError?.code === 404) {
        return undefined;
      }

      throw new Error(`failed to get lockfile metadata: ${apiError.message}`);
    }
  }

  async createLock(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    lockMetadata: LockMetadata,
  ) {
    // try to create the lockfile
    try {
      await this.uploadFile(bucket, localFilePath, destFilename, { ...lockMetadata });
      return;
    } catch (err) {
      const apiError = err as ApiError;
      if (apiError?.code !== 412) {
        throw new Error(`failed to create lockfile: ${apiError.message}`);
      }
    }

    // failed to write lockfile with 412 precondition failed (meaning lockfile exists)
    // see if we already own the lock
    let remoteMetadata;
    try {
      remoteMetadata = await this.getMetadata(bucket, destFilename);
    } catch (err) {
      const apiError = err as ApiError;
      throw new Error(`failed to get lockfile metadata: ${apiError.message}`);
    }

    if (remoteMetadata.metadata.lockId !== lockMetadata.lockId) {
      throw new Error(`Plan is locked by another process: ${remoteMetadata.metadata.lockRepoURL}`);
    }
  }

  async validateLock(
    bucket: string,
    filename: string,
    lockId: string,
    expectLockfile: boolean = true,
  ): Promise<void> {
    try {
      const remoteMetadata = await this.getMetadata(bucket, filename, true);
      if (expectLockfile && !remoteMetadata) {
        throw new Error(`Lockfile does not exist.`);
      }

      if (remoteMetadata.metadata.lockId !== lockId) {
        throw new Error(
          `Plan is locked by another process: ${remoteMetadata.metadata.lockRepoURL}`,
        );
      }
    } catch (err) {
      const apiError = err as ApiError;
      throw new Error(`failed to validate lockfile: ${apiError.message}`);
    }
  }

  async removeLock(
    bucket: string,
    filename: string,
    lockId: string,
    ignoreNotFound: boolean = false,
  ): Promise<void> {
    try {
      const remoteMetadata = await this.getMetadata(bucket, filename, ignoreNotFound);
      if (!remoteMetadata) {
        return;
      }

      if (remoteMetadata.metadata.lockId !== lockId) {
        throw new Error(
          `Plan is locked by another process: ${remoteMetadata.metadata.lockRepoURL}`,
        );
      }

      await this.deleteFile(bucket, filename);
    } catch (err) {
      const apiError = err as ApiError;
      throw new Error(`failed to remove lockfile: ${apiError.message}`);
    }
  }
}

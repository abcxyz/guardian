import { LockMetadata, StorageClient } from '../../src/common/storage';

export class MockStorageClient implements StorageClient {
  constructor() {}
  uploadFile(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    metadata: { [key: string]: string },
  ): Promise<void> {
    return Promise.resolve();
  }
  downloadFile(bucket: string, remoteFilename: string, destFilePath: string): Promise<void> {
    return Promise.resolve();
  }

  deleteFiles(bucket: string, prefix: string): Promise<void> {
    return Promise.resolve();
  }
  deleteFile(bucket: string, filename: string): Promise<void> {
    return Promise.resolve();
  }
  getMetadata(bucket: string, filename: string): Promise<any> {
    return Promise.resolve();
  }
  createLock(
    bucket: string,
    localFilePath: string,
    destFilename: string,
    lockMetadata: LockMetadata,
  ): Promise<void> {
    return Promise.resolve();
  }
  validateLock(
    bucket: string,
    filename: string,
    lockId: string,
    expectLockfile: boolean,
  ): Promise<void> {
    return Promise.resolve();
  }
  removeLock(bucket: string, filename: string, lockId: string): Promise<void> {
    return Promise.resolve();
  }
}

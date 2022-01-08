export interface GRPCExecutor {
  invoke<I, O>(name: string, args: I): Promise<O>;
  stream<I, O>(name: string, args: I, cb: (data: O) => unknown): Promise<void>;
}

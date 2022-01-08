import { GRPCExecutor } from "@sdk/grpc.ts";

export class ClientError extends Error {
}

export class GRPCClient implements GRPCExecutor {
  constructor(private endpoint: string) {}

  async invoke<I, O>(name: string, args: I): Promise<O> {
    const res = await fetch(`http://${this.endpoint}/call/${name}`, {
      method: "POST",
      body: JSON.stringify(args),
    });

    if (res.status !== 200) {
      throw new ClientError(
        `Failed to execute gRPC request: ${await res.text()}`,
      );
    }

    return await res.json();
  }

  stream<I, O>(name: string, args: I, cb: (data: O) => unknown): Promise<void> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(`ws://${this.endpoint}/stream/${name}`);
      socket.onopen = (e) => {
        socket.send(JSON.stringify(args));
      };
      socket.onmessage = async (e) => {
        const data = JSON.parse(e.data);
        await cb(data as O);
      };
      socket.onclose = (e) => {
        resolve();
      };
      socket.onerror = (e) => {
        reject();
      };
    });
  }
}

# Dynamic Mission System

DMS is a dynamic TypeScript environment designed for remote Digital Combat
Simulator scripting, based off [DCS-gRPC](https://github.com/DCS-gRPC).

## Example

```typescript
import { GRPCClient } from "@sdk/api/client.ts";
import { HookService } from "@sdk/gen/dcs/hook/v0.ts";
import { UnitStore } from "@sdk/stores/unit.ts";

async function main() {
  const client = new GRPCClient("localhost:6975");
  const hookService = new HookService(client);
  const res = await hookService.Eval({
    lua: "return 1 + 1",
  });
  console.log(JSON.parse(res.json));

  const unitStore = new UnitStore();
  unitStore.run(client);
  unitStore.onUnitSpawn = (u) => {
    console.log(`new unit spawned: ${u.id} (${u.name})`);
  };
}
main();
```

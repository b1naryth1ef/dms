import { GRPCClient } from "@sdk/api/client.ts";
import { UnitService } from "@sdk/gen/dcs/unit/v0.ts";
import { SmokeRequest, TriggerService } from "@sdk/gen/dcs/trigger/v0.ts";

async function main() {
  const client = new GRPCClient("localhost:6975");
  const unitService = new UnitService(client);

  const unit = await unitService.Get({
    name: "1NZA F16",
  });
  console.log(unit);

  const desc = await unitService.GetDescriptor({
    name: "1NZA F16",
  });
  console.log(desc);

  const triggerService = new TriggerService(client);
  await triggerService.Smoke({
    position: { lat: 42.183607661971, lon: 42.48219002461279, alt: 46.8515625 },
    color: SmokeRequest.SmokeColor.RED,
  });
}

main();

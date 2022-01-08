import { Unit } from "@sdk/gen/dcs/common/v0.ts";
import { GRPCClient } from "@sdk/api/client.ts";
import { MissionService } from "@sdk/gen/dcs/mission/v0.ts";

export class UnitStore {
  private units: Map<number, Unit> = new Map();

  onUnitSpawn?: (e: Unit) => unknown;
  onUnitUpdate?: (previous: Unit, next: Unit) => unknown;
  onUnitDespawn?: (previous: Unit) => unknown;

  all(): Array<Unit> {
    return [...this.units.values()];
  }

  run(client: GRPCClient) {
    const missionService = new MissionService(client);
    missionService.StreamUnits({}, (e) => {
      if ("unit" in e) {
        if (this.units.has(e.unit.id)) {
          if (this.onUnitUpdate) {
            this.onUnitUpdate(this.units.get(e.unit.id)!, e.unit);
          }
        } else {
          if (this.onUnitSpawn) {
            this.onUnitSpawn(e.unit);
          }
        }
        this.units.set(e.unit.id, e.unit);
      } else {
        console.log(`Delete unit ${e.gone.id}`);
        if (this.onUnitDespawn && this.units.has(e.gone.id)) {
          this.onUnitDespawn(this.units.get(e.gone.id)!);
        }
        this.units.delete(e.gone.id);
      }
    }).then(() => {
      console.log("UnitStore connection closed, reopening");
      setTimeout(() => this.run(client), 2000);
    });
  }
}

"""
smart-factory-demo integration example.

Shows how to use go-factory-io from the FastAPI backend to:
1. Read equipment status and sensor data
2. Write equipment parameters
3. Send remote commands
4. Listen for real-time events and write to DB

Prerequisites:
    pip install httpx

Usage:
    # Start go-factory-io simulator first:
    ./bin/secsgem simulate --api :8080

    # Then run this script:
    python examples/smart-factory/integration.py
"""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../clients/python"))

from factory_io import FactoryIOSyncClient, FactoryIOError


def main():
    client = FactoryIOSyncClient("http://localhost:8080")

    try:
        # 1. Health check
        health = client.health()
        print(f"Health: {health}")

        # 2. Equipment status
        status = client.get_status()
        print(f"\nEquipment Status:")
        print(f"  Comm:    {status.comm_state}")
        print(f"  Control: {status.control_state}")
        print(f"  Online:  {status.online}")

        # 3. Read all status variables
        svs = client.list_sv()
        print(f"\nStatus Variables ({len(svs)}):")
        for sv in svs:
            print(f"  [{sv.svid}] {sv.name} = {sv.value} {sv.units}")

        # 4. Read specific SV (temperature)
        try:
            temp = client.get_sv(1002)
            print(f"\nTemperature: {temp.value:.1f} {temp.units}")
        except FactoryIOError:
            print("\nTemperature SV not available")

        # 5. Read equipment constants
        ecs = client.list_ec()
        print(f"\nEquipment Constants ({len(ecs)}):")
        for ec in ecs:
            print(f"  [{ec.ecid}] {ec.name} = {ec.value} {ec.units}")

        # 6. Update equipment constant
        print("\nSetting ProcessTemperature to 400.0...")
        client.set_ec(1, 400.0)
        updated = client.get_ec(1)
        print(f"  Updated: {updated.name} = {updated.value} {updated.units}")

        # 7. Check alarms
        alarms = client.list_alarms()
        print(f"\nAlarms ({len(alarms)}):")
        for a in alarms:
            print(f"  [{a.alid}] {a.name}: {a.state} (enabled={a.enabled})")

        # 8. Send remote command
        print("\nSending START command...")
        try:
            result = client.send_command("START", {"recipe": "PROCESS_A"})
            print(f"  Result: {result.status} (code={result.code})")
        except FactoryIOError as e:
            print(f"  Command failed: {e}")

    except Exception as e:
        print(f"Error: {e}")
        print("Make sure go-factory-io simulator is running: ./bin/secsgem simulate")
    finally:
        client.close()


# --- FastAPI integration example ---
# Add this to smart-factory-demo's routes:
#
# from factory_io import FactoryIOSyncClient
#
# factory_client = FactoryIOSyncClient(
#     base_url=os.getenv("FACTORY_IO_URL", "http://localhost:8080"),
#     token=os.getenv("FACTORY_IO_TOKEN"),
# )
#
# @router.get("/api/v1/equipment/live-status")
# def get_live_equipment_status():
#     status = factory_client.get_status()
#     svs = factory_client.list_sv()
#     return {
#         "equipment": {
#             "commState": status.comm_state,
#             "controlState": status.control_state,
#             "online": status.online,
#         },
#         "sensors": {sv.name: {"value": sv.value, "units": sv.units} for sv in svs},
#     }
#
# @router.post("/api/v1/equipment/command")
# def send_equipment_command(command: str, params: dict = {}):
#     result = factory_client.send_command(command, params)
#     return {"command": result.command, "status": result.status}


if __name__ == "__main__":
    main()

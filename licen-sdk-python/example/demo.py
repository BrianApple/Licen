"""licen-sdk-python 使用示例。

运行: python example/demo.py
环境变量: LICEN_SERVER_URL / LICEN_APP_KEY / LICEN_APP_SECRET
"""

import os
import time

from licen_sdk import LicenClient


def main() -> None:
    client = LicenClient(
        server_url=os.getenv("LICEN_SERVER_URL", "http://127.0.0.1:8090"),
        app_key=os.getenv("LICEN_APP_KEY", "ai-engine"),
        app_secret=os.getenv("LICEN_APP_SECRET", "licen-demo-secret-2026"),
        node_name="example-python-sdk",
    )

    def on_change(status):
        print(f"📢 授权状态变化: valid={status.valid} licenseId={status.license_id}")

    client.on_status_change(on_change)
    client.start()
    print(f"🚀 licen-sdk-python 已启动, nodeId={client.node_id}")

    for _ in range(6):  # 打印 6 次状态
        time.sleep(3)
        st = client.status()
        print(f"状态: valid={st.valid} degraded={client.is_degraded()} "
              f"license={st.license_id} 节点={st.online_nodes}/{st.max_nodes} 功能={st.features}")

    client.stop()
    print("已退出")


if __name__ == "__main__":
    main()

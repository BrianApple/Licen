"""licen_sdk - Licen 授权客户端 SDK（零第三方依赖）。

用法::

    from licen_sdk import LicenClient

    client = LicenClient(
        server_url="http://10.0.0.10:8090",
        app_key="ai-engine",
        app_secret="xxx",
    )
    client.start()
    client.is_valid()              # bool
    client.has_feature("ai-inference")  # bool
    client.stop()
"""

from .client import LicenClient, Status

__all__ = ["LicenClient", "Status"]
__version__ = "1.0.0"

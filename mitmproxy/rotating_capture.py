# rotating_capture.py
from mitmproxy import http
from mitmproxy.io import FlowWriter

class RotatingCapture:
    def __init__(self):
        self.max_size_mb = 50          # rotate after 50 MB
        self.output_dir = "captured"
        self.file_index = 0
        self.current_file = None
        self.writer = None

        import os
        os.makedirs(self.output_dir, exist_ok=True)
        self._open_new_file()

    def _open_new_file(self):
        if self.current_file:
            self.current_file.close()
        filename = f"{self.output_dir}/traffic_{self.file_index:04d}.mitm"
        self.current_file = open(filename, "wb")
        self.writer = FlowWriter(self.current_file)
        print(f"[rotate] Writing to {filename}")
        self.file_index += 1

    def response(self, flow: http.HTTPFlow):
        size_mb = self.current_file.tell() / (1024 * 1024)
        if size_mb >= self.max_size_mb:
            self._open_new_file()
        self.writer.add(flow)

addons = [RotatingCapture()]

import io
import os
import sys
import tempfile
import types
import unittest
from unittest import mock

import updater


class UpdaterBearerTokenTests(unittest.TestCase):
    def setUp(self):
        self.secrets = types.ModuleType("secrets")
        self.secrets.TINY_UPDATER_BEARER_TOKEN = "test-token"
        self.secrets_patch = mock.patch.dict(sys.modules, {"secrets": self.secrets})
        self.secrets_patch.start()
        self.addCleanup(self.secrets_patch.stop)

    def response(self):
        response = mock.Mock()
        response.status_code = 200
        response.text = '{"version":"1"}'
        response.raw = io.BytesIO(b"file contents")
        return response

    def test_manifest_request_includes_bearer_token(self):
        with mock.patch.object(updater.requests, "get", return_value=self.response()) as get:
            updater.fetch_manifest("owner/repo", host="example.test")

        self.assertEqual(
            get.call_args.kwargs["headers"],
            {"Authorization": "Bearer " + self.secrets.TINY_UPDATER_BEARER_TOKEN},
        )

    def test_download_request_includes_bearer_token(self):
        response = self.response()
        with tempfile.TemporaryDirectory() as directory:
            current_directory = os.getcwd()
            os.chdir(directory)
            try:
                with mock.patch.object(updater.requests, "get", return_value=response) as get:
                    updater.download_file("owner/repo", "file.txt", "file.txt", host="example.test")

                self.assertEqual(
                    get.call_args.kwargs["headers"],
                    {"Authorization": "Bearer " + self.secrets.TINY_UPDATER_BEARER_TOKEN},
                )
                with open("file.txt", "rb") as downloaded:
                    self.assertEqual(downloaded.read(), b"file contents")
            finally:
                os.chdir(current_directory)

    def test_requests_omit_authorization_when_token_is_missing(self):
        missing_token_secrets = types.ModuleType("secrets")
        with mock.patch.dict(sys.modules, {"secrets": missing_token_secrets}):
            with mock.patch.object(updater.requests, "get", return_value=self.response()) as get:
                updater.fetch_manifest("owner/repo", host="example.test")

            self.assertEqual(get.call_args.kwargs["headers"], {})

            with tempfile.TemporaryDirectory() as directory:
                current_directory = os.getcwd()
                os.chdir(directory)
                try:
                    with mock.patch.object(updater.requests, "get", return_value=self.response()) as get:
                        updater.download_file(
                            "owner/repo", "file.txt", "file.txt", host="example.test"
                        )

                    self.assertEqual(get.call_args.kwargs["headers"], {})
                finally:
                    os.chdir(current_directory)


if __name__ == "__main__":
    unittest.main()

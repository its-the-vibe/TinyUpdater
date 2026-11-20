.PHONY: bootstrap

bootstrap:
	mpremote fs cp secrets.py :secrets.py
	mpremote mip install github:its-the-vibe/TinyUpdater/updater.py
	mpremote run bootstrap.py

# legnaris

A new Flutter project.

## Linux prerequisites

`flutter run -d linux` / `flutter build linux` requires the Secret Service
library because of the secure refresh-token store (`flutter_secure_storage`).
Without it the CMake configure step fails with `libsecret-1>=0.18.4` not found
and the app never launches. Install it once:

```bash
sudo apt update && sudo apt install -y libsecret-1-dev
```

At runtime `libsecret` also needs a Secret Service (e.g. `gnome-keyring`) over
D-Bus; where it is absent (bare containers/WSL) persistence is skipped and the
session stays in memory — login still works.

## Getting Started

This project is a starting point for a Flutter application.

A few resources to get you started if this is your first Flutter project:

- [Learn Flutter](https://docs.flutter.dev/get-started/learn-flutter)
- [Write your first Flutter app](https://docs.flutter.dev/get-started/codelab)
- [Flutter learning resources](https://docs.flutter.dev/reference/learning-resources)

For help getting started with Flutter development, view the
[online documentation](https://docs.flutter.dev/), which offers tutorials,
samples, guidance on mobile development, and a full API reference.

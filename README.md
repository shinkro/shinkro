<h1 align="center">
  <img alt="shinkro logo" src=".github/images/logo.png" width="160px"/><br/>
  shinkro
</h1>

<p align="center">An application to sync Plex watch status to myanimelist.</p>

<p align="center"><img alt="GitHub release (latest by date)" src="https://img.shields.io/github/v/release/shinkro/shinkro?style=for-the-badge">&nbsp;<img alt="GitHub all releases" src="https://img.shields.io/github/downloads/shinkro/shinkro/total?style=for-the-badge">&nbsp;<img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/shinkro/shinkro/release.yml?style=for-the-badge"></p>

## Documentation

Installation guide and documentation can be found at https://docs.shinkro.com

## Key features

- Support for multiple metadata agents, including default Plex agent, HAMA, and MyAnimeList.bundle.
- Live updates to myanimelist as soon as you watch or rate in Plex.
- Powerful anime-id mapping support, make custom maps or use the community mapping.
- Built on Go & React making shinkro lightweight and perfect for supporting multiple platforms (Linux, FreeBSD,
  Windows, macOS) on different architectures. (e.g. x86, ARM)
- Discord & Gotify Notifications.
- Base path / Subfolder (and subdomain) support for convenient reverse-proxy support.

Available methods to use shinkro

- Official Plex Webhook (Requires Plex Pass) (recommended)
- Tautulli (Limitation: cannot sync ratings)

## What is shinkro?

If you use both Plex and Myanimelist to watch and track your anime, you know how mundande and boring it is to have to update your myanimelist manually after watching an anime on your Plex server.

shinkro enables you to sync your Plex ratings and watch status for anime to myanimelist.net.

## Installation

Full installation guide and documentation can be found at https://docs.shinkro.com/installation

### Quickstart via Docker:

```
docker run \
    --name shinkro \
    -v /path/to/shinkro/config:/config \
    -e TZ=US/Pacific \
    -p 7011:7011 \
    --restart unless-stopped \
    ghcr.io/shinkro/shinkro:latest
```

## Custom Mapping

While shinkro maps most malids to tvdbids in it's database it only works well for season 1 of anime. Multiseason anime mapping is too complicated to automate at this point in time. For malid to tmdbids, a lot of movies are properly mapped in shinkro's database but not all of them. The ones which aren't are listed in [community-mapping](https://github.com/shinkro/community-mapping) ready for manual mapping.

By default, shinkro will use the community mapping hosted in the [community-mapping](https://github.com/shinkro/community-mapping) repository. It is encouraged for the user base to use the community mapping - if it does not contain a mapping you need, consider contributing or creating an issue. 

Of course, you do have the option to specify your own custom maps. Simply place `tvdb-mal.yaml` for MAL-TVDB mappings or `tmdb-mal.yaml` for MAL-TMDB mappings in shinkro's configuration directory (where config.toml and shinkro.db files are located). shinkro will automatically detect the change and start using your custom mapping(s). The structure of both yaml files can be viewed at the [community-mapping](https://github.com/shinkro/community-mapping) repository.

## Community

Come join us on [Discord](https://discord.gg/ZkYdfNgbAT)!

## License

* [MIT](https://mit-license.org/)
* Copyright 2022-2025
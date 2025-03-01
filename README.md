# Homemon Command Line Usage Documentation

Homemon is a command-line tool for monitoring environmental metrics using Netatmo devices. This documentation provides detailed instructions for configuring, using, and managing the Homemon application, enabling comprehensive home monitoring through various command-line operations.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Building the Project](#building-the-project)
3. [Command Structure](#command-structure)
4. [Global Options](#global-options)
5. [Command Reference](#command-reference)
   - [Netatmo Commands](#netatmo-commands)
   - [Metrics Commands](#metrics-commands)
   - [Cleanup Commands](#cleanup-commands)
6. [Usage Examples](#usage-examples)
7. [Access Token Management](#access-token-management)
8. [Configuration](#configuration)
9. [Sample Configuration File](#sample-configuration-file)

## Quick Start

You can execute Homemon using Nix flakes without cloning the repository:

```bash
nix run github:venkytv/homemon
```

## Building the Project

To build Homemon from the source, use the following commands:

```bash
git clone https://github.com/venkytv/homemon.git
cd homemon
go build
```

After building, you can run the application using:

```bash
./homemon
```

## Command Structure

Homemon uses a structured command-line interface that follows this syntax:

```bash
homemon [global options] command [command options] [arguments...]
```

## Global Options

These options can be used with any command to configure the application's behavior:

- `--config-dir <directory>`: Specify the directory for configuration files (default is `~/.config/homemon`).
- `--redis-address <address>`: Set the Redis server address (default is `localhost:6379`).
- `--nats-address <address>`: Set the NATS server address (default is `localhost:4222`).
- `--debug`: Enables debug mode for detailed logging (default is false).

## Command Reference

### Netatmo Commands

#### `netatmo record-metrics`

Activates a service that records metrics from Netatmo devices at predefined intervals.

- **Usage:**
  ```bash
  homemon netatmo record-metrics
  ```

### Metrics Commands

#### `metrics publish`

Allows publishing of a metric with specific attributes, including name, priority, color, and TTL, useful for testing.

- **Options:**
  - `--name, -n <name>`: Metric name (required).
  - `--priority, -p <priority>`: Metric priority level (required).
  - `--colour, -c <colour>`: Metric color code (required).
  - `--ttl, -t <duration>`: Metric's time-to-live (TTL), specified as a duration (required).

- **Usage:**
  ```bash
  homemon metrics publish --name temperature --priority 10 --colour red --ttl 1h
  ```

#### `metrics list`

Displays all metrics currently stored.

- **Usage:**
  ```bash
  homemon metrics list
  ```

#### `metrics delete`

Removes a specified metric by its name.

- **Usage:**
  ```bash
  homemon metrics delete <metric_name>
  ```

### Cleanup Commands

#### `cleanup metrics`

Purges expired metrics from the database. A dry-run option is available to preview the cleanup process without execution.

- **Options:**
  - `--dry-run`: Simulate cleanup without deletion.

- **Usage:**
  ```bash
  homemon cleanup metrics --dry-run
  ```

## Usage Examples

- **Start Netatmo metrics recording:**
  ```bash
  homemon netatmo record-metrics --debug
  ```

- **Publish a new metric:**
  ```bash
  homemon metrics publish --name humidity --priority 20 --colour blue --ttl 2h
  ```

- **List all existing metrics:**
  ```bash
  homemon metrics list
  ```

- **Delete a specific metric:**
  ```bash
  homemon metrics delete humidity
  ```

- **Execute a dry-run cleanup for metrics:**
  ```bash
  homemon cleanup metrics --dry-run
  ```

## Access Token Management

Homemon utilizes the Netatmo API, which requires OAuth tokens. Refresh tokens are stored securely and used to obtain access tokens. The tokens are refreshed automatically by `homemon` as needed.

For NetAtmo, you need to do the following:
1. Create a developer account: https://dev.netatmo.com/apidocumentation
2. Create an application: https://dev.netatmo.com/apps/createanapp#form

Once the app is created, you need the client ID, client secret, and the refresh token. (You can ignore the access token. `homemon` will use the refresh token to create a new access token.)

## Configuration

Configuration files define how `homemon` interacts with various services and manage internal data. Key files such as `netatmo-config.yaml` store mappings of rooms to device IDs and define metric thresholds.

Environment variables `NETATMO_CLIENT_ID` and `NETATMO_CLIENT_SECRET` must be set for OAuth handling. Also, the refresh token should be saved in the file: `~/.config/homemon/netatmo-refresh-token`:

```bash
$ cat ~/.config/homemon/netatmo-refresh-token
678abc123|def64775...
```

## Sample Configuration File

Here's an example configuration file `netatmo-config.yaml` that you need to include in your configuration directory (`~/.config/homemon`):

(You can get the MAC addresses for your NetAtmo Indoor Air Quality Monitor from your "Home Coach" app. Look in "Settings > Advanced Settings > Your Device".)

```yaml
mac-ids:
  bedroom: 70:ee:12:34:56:78
  livingroom: 70:ee:12:34:56:79

metrics:
  humidity:
    - from: 0
      to: 30
      priority: 70
      colour: red
    - from: 60
      to: 70
      priority: 50
      colour: lightblue
    - from: 70
      to: 100
      priority: 80
      colour: blue

  temperature:
    - from: -100
      to: 17
      priority: 40
      colour: blue
    - from: 24
      to: 100
      priority: 45
      colour: red

  co2:
    - from: 900
      to: 1200
      priority: 25
      colour: yellow
    - from: 1200
      to: 1400
      priority: 75
      colour: pink
    - from: 1400
      to: 100000
      priority: 85
      colour: red

  noise:
    - from: 60
      to: 10000
      priority: 35
      colour: yellow
```

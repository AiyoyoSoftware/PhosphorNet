#!/bin/sh
set -eu

usage() {
	cat <<'USAGE'
Usage:
  install.sh (--client | --node | --full)
  install.sh --uninstall (--client | --node | --full) [--purge]

Modes:
  --client  install phosphor only
  --node    install phosphord, phosphor-actiond, bundled doors, and node state/config
  --full    install phosphor, phosphord, phosphor-actiond, switchboard, bundled doors, and node state/config
  --uninstall  remove installed binaries and bundled doors
  --purge      with --uninstall, also remove node config and database state

Environment overrides:
  PHOSPHORNET_BIN_DIR       default: /usr/local/bin
  PHOSPHORNET_SHARE_DIR     default: /usr/local/share/phosphornet
  PHOSPHORNET_CONFIG_DIR    default: /etc/phosphornet
  PHOSPHORNET_STATE_DIR     default: /var/lib/phosphornet
  PHOSPHORNET_ARTIFACT_DIR  directory containing phosphor/phosphord/phosphor-actiond/switchboard and doors/
  PHOSPHORNET_ARTIFACT_URL  exact release archive URL to install
  PHOSPHORNET_SOURCE_DIR    explicit source checkout to build from instead of downloading release artifacts
  PHOSPHORNET_VERSION       release version, default: latest
  PHOSPHORNET_RELEASE_BASE_URL  release asset base URL
  PHOSPHORNET_STATION_NAME  station name for first node config, default: localbox
USAGE
}

mode=""
uninstall=false
purge=false
while [ "$#" -gt 0 ]; do
	case "$1" in
		--client|--node|--full)
			if [ -n "$mode" ]; then
				echo "choose only one install mode" >&2
				usage >&2
				exit 2
			fi
			mode="${1#--}"
			;;
		--uninstall)
			uninstall=true
			;;
		--purge)
			purge=true
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			echo "unknown argument: $1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

if [ -z "$mode" ]; then
	echo "missing install mode" >&2
	usage >&2
	exit 2
fi
if [ "$purge" = true ] && [ "$uninstall" != true ]; then
	echo "--purge can only be used with --uninstall" >&2
	usage >&2
	exit 2
fi

prefix="${PREFIX:-/usr/local}"
bin_dir="${PHOSPHORNET_BIN_DIR:-$prefix/bin}"
share_dir="${PHOSPHORNET_SHARE_DIR:-$prefix/share/phosphornet}"
config_dir="${PHOSPHORNET_CONFIG_DIR:-/etc/phosphornet}"
state_dir="${PHOSPHORNET_STATE_DIR:-/var/lib/phosphornet}"
config_path="$config_dir/node.toml"
actiond_config_path="$config_dir/actiond.toml"
station_name="${PHOSPHORNET_STATION_NAME:-localbox}"
operator_uid="${SUDO_UID:-}"
operator_gid="${SUDO_GID:-}"
operator_home=""
if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
	operator_home="$(getent passwd "$SUDO_USER" 2>/dev/null | cut -d: -f6 || true)"
fi
if [ -n "${PHOSPHORNET_ADMIN_PASSPORT:-}" ]; then
	admin_passport="$PHOSPHORNET_ADMIN_PASSPORT"
elif [ -n "$operator_home" ]; then
	admin_passport="$operator_home/.config/phosphornet/passport.toml"
else
	admin_passport="$HOME/.config/phosphornet/passport.toml"
fi

using_default_system_paths=false
if [ "$bin_dir" = "/usr/local/bin" ] || [ "$share_dir" = "/usr/local/share/phosphornet" ] || [ "$config_dir" = "/etc/phosphornet" ] || [ "$state_dir" = "/var/lib/phosphornet" ]; then
	using_default_system_paths=true
fi

if [ "$using_default_system_paths" = true ] && [ "$(id -u)" -ne 0 ]; then
	echo "PhosphorNet installs to system paths by default:" >&2
	echo "  $bin_dir" >&2
	echo "  $share_dir" >&2
	echo "  $config_dir" >&2
	echo "  $state_dir" >&2
	echo "Re-run as root, for example:" >&2
	echo "  curl -fsSL https://aiyoyo.org/phosphornet/install.sh | sudo sh -s -- --$mode" >&2
	exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

parent_dir() {
	dirname "$1"
}

run_privileged() {
	"$@"
}

ensure_dir() {
	path="$1"
	parent="$(parent_dir "$path")"
	if [ -d "$path" ]; then
		return 0
	fi
	if [ -w "$parent" ]; then
		mkdir -p "$path"
	else
		run_privileged mkdir -p "$path"
	fi
}

install_file() {
	mode_bits="$1"
	src="$2"
	dst="$3"
	ensure_dir "$(parent_dir "$dst")"
	if [ -w "$(parent_dir "$dst")" ]; then
		install -m "$mode_bits" "$src" "$dst"
	else
		run_privileged install -m "$mode_bits" "$src" "$dst"
	fi
}

remove_path() {
	path="$1"
	if [ ! -e "$path" ]; then
		return 0
	fi
	parent="$(parent_dir "$path")"
	if [ -w "$parent" ]; then
		rm -rf "$path"
	else
		run_privileged rm -rf "$path"
	fi
}

chown_operator_path() {
	path="$1"
	if [ -n "$operator_uid" ] && [ -n "$operator_gid" ] && [ -e "$path" ]; then
		chown -R "$operator_uid:$operator_gid" "$path"
	fi
}

copy_tree() {
	src="$1"
	dst="$2"
	ensure_dir "$dst"
	if [ -w "$dst" ]; then
		( cd "$src" && tar -cf - . ) | ( cd "$dst" && tar -xf - )
	else
		tar_file="$tmp_dir/tree.tar"
		( cd "$src" && tar -cf "$tar_file" . )
		run_privileged tar -xf "$tar_file" -C "$dst"
	fi
}

host_os() {
	uname -s | tr '[:upper:]' '[:lower:]'
}

host_arch() {
	case "$(uname -m)" in
		x86_64|amd64) echo "amd64" ;;
		aarch64|arm64) echo "arm64" ;;
		*) uname -m ;;
	esac
}

build_from_source() {
	source_dir="$1"
	out_dir="$tmp_dir/artifacts"
	mkdir -p "$out_dir"
	for binary in "$@"; do
		if [ "$binary" = "$source_dir" ]; then
			continue
		fi
		( cd "$source_dir" && go build -o "$out_dir/$binary" "./cmd/$binary" )
	done
	if [ -d "$source_dir/doors" ]; then
		cp -R "$source_dir/doors" "$out_dir/doors"
	fi
	echo "$out_dir"
}

download_release() {
	version="${PHOSPHORNET_VERSION:-latest}"
	base_url="${PHOSPHORNET_RELEASE_BASE_URL:-https://github.com/AiyoyoSoftware/PhosphorNet/releases/download}"
	os_name="$(host_os)"
	arch_name="$(host_arch)"
	package="phosphornet_${os_name}_${arch_name}"
	archive="$tmp_dir/phosphornet.tar.gz"
	if [ "$version" = "latest" ]; then
		default_url="https://github.com/AiyoyoSoftware/PhosphorNet/releases/latest/download/$package.tar.gz"
	else
		default_url="$base_url/$version/$package.tar.gz"
	fi
	url="${PHOSPHORNET_ARTIFACT_URL:-$default_url}"
	curl -fsSL "$url" -o "$archive"
	mkdir -p "$tmp_dir/release"
	tar -xzf "$archive" -C "$tmp_dir/release"
	if [ -d "$tmp_dir/release/$package" ]; then
		echo "$tmp_dir/release/$package"
	else
		echo "$tmp_dir/release"
	fi
}

artifact_binary() {
	root="$1"
	name="$2"
	if [ -x "$root/$name" ]; then
		echo "$root/$name"
		return 0
	fi
	if [ -x "$root/bin/$name" ]; then
		echo "$root/bin/$name"
		return 0
	fi
	echo "artifact missing executable $name" >&2
	return 1
}

binaries=""
case "$mode" in
	client) binaries="phosphor" ;;
	node) binaries="phosphord phosphor-actiond" ;;
	full) binaries="phosphor phosphord phosphor-actiond switchboard" ;;
esac

if [ "$uninstall" = true ]; then
	for binary in $binaries; do
		remove_path "$bin_dir/$binary"
		echo "removed $bin_dir/$binary"
	done
	case "$mode" in
		node|full)
			remove_path "$share_dir/doors"
			echo "removed $share_dir/doors"
			if [ "$purge" = true ]; then
				remove_path "$config_path"
				remove_path "$actiond_config_path"
				remove_path "$state_dir/phosphornet.db"
				remove_path "$state_dir/phosphornet.db-shm"
				remove_path "$state_dir/phosphornet.db-wal"
				echo "purged $config_path, $actiond_config_path, and $state_dir/phosphornet.db"
			else
				echo "kept $config_path, $actiond_config_path, and $state_dir/phosphornet.db"
			fi
			;;
	esac
	echo "PhosphorNet $mode uninstall complete."
	exit 0
fi

if [ -n "${PHOSPHORNET_ARTIFACT_DIR:-}" ]; then
	artifact_dir="$PHOSPHORNET_ARTIFACT_DIR"
elif [ -n "${PHOSPHORNET_SOURCE_DIR:-}" ]; then
	artifact_dir="$(build_from_source "$PHOSPHORNET_SOURCE_DIR" $binaries)"
else
	artifact_dir="$(download_release)"
fi

ensure_dir "$bin_dir"
for binary in $binaries; do
	install_file 0755 "$(artifact_binary "$artifact_dir" "$binary")" "$bin_dir/$binary"
	echo "installed $bin_dir/$binary"
done

case "$mode" in
	node|full)
		ensure_dir "$share_dir"
		ensure_dir "$state_dir"
		if [ -d "$artifact_dir/doors" ]; then
			copy_tree "$artifact_dir/doors" "$share_dir/doors"
			echo "installed $share_dir/doors"
		else
			echo "warning: artifact has no bundled doors directory" >&2
		fi

		ensure_dir "$config_dir"
		if [ ! -f "$config_path" ]; then
			if [ -w "$config_dir" ] && [ -w "$state_dir" ]; then
				"$bin_dir/phosphord" init --name "$station_name" --out "$config_path" --admin-passport "$admin_passport" --system-paths
			else
				run_privileged "$bin_dir/phosphord" init --name "$station_name" --out "$config_path" --admin-passport "$admin_passport" --system-paths
			fi
		else
			echo "kept existing $config_path"
		fi
		chown_operator_path "$config_path"
		chown_operator_path "$state_dir"
		chown_operator_path "$admin_passport"
		chown_operator_path "$(parent_dir "$admin_passport")"
		;;
esac

echo "PhosphorNet $mode install complete."

# Cutover window: load named keys from an env file WITHOUT evaluating it (bash).
#   source load-env.sh; load_env_file <file> KEY [KEY...]
# For each key: the first line `KEY=...` (an optional leading `export ` is allowed) is split at the
# first '=', a trailing CR is dropped, one pair of matching surrounding quotes ('...' or "...") is
# stripped, and the rest is exported byte for byte: no expansion, no eval, no word splitting.
# A missing key is skipped (the runbook's defaults apply). Prints only the names it loaded.
load_env_file() {
  local file=$1 key line val loaded=""
  shift
  [ -r "$file" ] || { echo "load_env_file: cannot read the env file" >&2; return 1; }
  for key in "$@"; do
    line=$(LC_ALL=C /usr/bin/grep -m1 -E "^(export[[:space:]]+)?${key}=" "$file") || continue
    val=${line#*=}
    val=${val%$'\r'}
    if [ "${#val}" -ge 2 ]; then
      case "$val" in
        \"*\") val=${val:1:${#val}-2} ;;
        \'*\') val=${val:1:${#val}-2} ;;
      esac
    fi
    export "$key=$val"
    loaded="$loaded $key"
  done
  echo "loaded:${loaded:- none}"
}

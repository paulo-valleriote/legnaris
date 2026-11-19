#!/usr/bin/env bash
# Claude Code PreToolUse guard for Bash. Reads the hook payload on stdin and
# denies commands that print .env or expand OPENCODE_PR into output, so a
# secret never reaches the transcript. Exit 2 blocks the tool call and shows
# stderr to the agent. Wired in the versioned .claude/settings.json.
set -euo pipefail

readonly SANCTIONED="Read OPENCODE_PR only into an Authorization header or a push URL, never into output."

readonly READER_COMMANDS=" cat bat batcat less more head tail nl tac xxd od strings grep rg egrep fgrep awk sed cut base64 sort uniq wc file "
readonly WRAPPER_COMMANDS=" sudo doas command nohup exec time env "

deny() {
  printf 'guard-secrets: %s\n' "$1" >&2
  printf 'guard-secrets: %s\n' "$SANCTIONED" >&2
  exit 2
}

is_listed() {
  local list=$1 word=$2
  [[ $list == *" $word "* ]]
}

is_reader() {
  is_listed "$READER_COMMANDS" "$1"
}

is_wrapper() {
  is_listed "$WRAPPER_COMMANDS" "$1"
}

is_dotenv_file() {
  local base=${1##*/}
  case $base in
    .env) return 0 ;;
    .env.example | .env.sample | .env.template | .env.dist | .env.defaults) return 1 ;;
    .env.*) return 0 ;;
  esac
  return 1
}

has_count_flag() {
  local word
  for word in "$@"; do
    case $word in
      --count | --count=*) return 0 ;;
    esac
    if [[ $word =~ ^-[A-Za-z]*c[A-Za-z]*$ ]]; then
      return 0
    fi
  done
  return 1
}

counts_matches_only() {
  case $1 in
    grep | egrep | fgrep | rg) return 0 ;;
  esac
  return 1
}

segment_is_sanctioned() {
  local segment=$1 occurrences
  occurrences=$(printf '%s' "$segment" | grep -o 'OPENCODE_PR' | wc -l | tr -d ' ')
  [[ $occurrences == 1 ]] || return 1
  if [[ $segment =~ (-H|--header)[[:space:]]+[\"\']?[^\"\']*[Aa]uthorization[^\"\']*OPENCODE_PR ]]; then
    return 0
  fi
  if [[ $segment =~ ://[^[:space:]]*OPENCODE_PR[^[:space:]]*@ ]]; then
    return 0
  fi
  return 1
}

split_segments() {
  printf '%s\n' "$1" | sed -E 's/(\|\||&&|[;|])/\n/g'
}

unquote() {
  local word=$1
  word=${word//\"/}
  word=${word//\'/}
  printf '%s' "$word"
}

check_environment_dump() {
  local -a words=()
  local word
  for word in "$@"; do
    word=$(unquote "$word")
    if [[ $word =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
      continue
    fi
    words+=("$word")
  done
  ((${#words[@]} > 0)) || return 0

  local command=${words[0]##*/}
  local length=${#words[@]}
  case $command in
    set | printenv)
      if ((length == 1)); then
        deny "refusing to dump the whole environment through $command"
      fi
      ;;
    env)
      local index=1 nested
      while ((index < length)) && [[ ${words[index]} == -* || ${words[index]} =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; do
        index=$((index + 1))
      done
      if ((index == length)); then
        deny "refusing to dump the whole environment through env"
      fi
      nested=${words[index]##*/}
      if ((index == length - 1)) && [[ $nested == printenv || $nested == set ]]; then
        deny "refusing to dump the whole environment through env $nested"
      fi
      ;;
  esac
}

check_segment() {
  local segment=$1 final=$2
  local -a words
  read -r -a words <<< "$segment"

  if ((${#words[@]} > 0)) && ((final)); then
    check_environment_dump "${words[@]}"
  fi

  local index=0 word cmd="" wrapped=0
  while ((index < ${#words[@]})); do
    word=$(unquote "${words[index]}")
    index=$((index + 1))
    if [[ $word =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
      continue
    fi
    if [[ $word == -* ]]; then
      continue
    fi
    if is_wrapper "${word##*/}"; then
      wrapped=1
      continue
    fi
    cmd=${word##*/}
    break
  done

  local -a args=()
  local dotenv=""
  while ((index < ${#words[@]})); do
    word=$(unquote "${words[index]}")
    index=$((index + 1))
    args+=("$word")
    if [[ -z $dotenv ]] && is_dotenv_file "$word"; then
      dotenv=$word
    fi
  done

  if [[ -n $dotenv && -n $cmd ]] && is_reader "$cmd"; then
    if counts_matches_only "$cmd" && ((${#args[@]} > 0)) && has_count_flag "${args[@]}"; then
      :
    else
      deny "refusing to read $dotenv: it holds live secrets"
    fi
  fi

  if ((wrapped)); then
    local candidate=""
    for word in "${words[@]}"; do
      word=$(unquote "$word")
      if [[ -z $candidate ]] && is_reader "${word##*/}"; then
        candidate=${word##*/}
      fi
      if [[ -z $dotenv ]] && is_dotenv_file "$word"; then
        dotenv=$word
      fi
    done
    if [[ -n $candidate && -n $dotenv ]]; then
      if counts_matches_only "$candidate" && has_count_flag "${words[@]}"; then
        :
      else
        deny "refusing to read $dotenv: it holds live secrets"
      fi
    fi
  fi

  if [[ $segment == *OPENCODE_PR* ]] && ! segment_is_sanctioned "$segment"; then
    deny "refusing to expand OPENCODE_PR outside the sanctioned header or push URL"
  fi
}

extract_payload() {
  local payload=$1
  if command -v python3 >/dev/null 2>&1; then
    printf '%s' "$payload" | python3 -c 'import json, sys
try:
    data = json.load(sys.stdin)
except Exception:
    raise SystemExit(1)
tool = data.get("tool_name") or ""
command = (data.get("tool_input") or {}).get("command") or ""
print(tool)
print(command)'
    return
  fi
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$payload" | jq -r '.tool_name? // empty, .tool_input.command? // empty'
    return
  fi
  return 1
}

payload=$(cat)
[[ -n ${payload//[[:space:]]/} ]] || exit 0

if ! parsed=$(extract_payload "$payload" 2>/dev/null); then
  echo "guard-secrets: neither python3 nor jq is available; skipping the secret guard" >&2
  exit 0
fi
tool=${parsed%%$'\n'*}
rest=${parsed#*$'\n'}
cmd=""
if [[ $rest != "$parsed" ]]; then
  cmd=$rest
fi
if [[ -n $tool && ${tool,,} != "bash" ]]; then
  exit 0
fi
[[ -n ${cmd//[[:space:]]/} ]] || exit 0

segments=()
while IFS= read -r segment; do
  [[ -n ${segment//[[:space:]]/} ]] || continue
  segments+=("$segment")
done < <(split_segments "$cmd")

for index in "${!segments[@]}"; do
  if ((index == ${#segments[@]} - 1)); then
    check_segment "${segments[index]}" 1
  else
    check_segment "${segments[index]}" 0
  fi
done

exit 0

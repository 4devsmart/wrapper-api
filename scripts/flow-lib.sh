# Carregado por todos os scripts do fluxo: vai para a raiz do repositorio e le
# a configuracao do projeto. Nao e executavel por si.
#
# Os valores abaixo sao os padroes; `.workflow.conf` na raiz sobrescreve o que o
# projeto precisar. Ver o comentario de cada um la.

cd "$(git rev-parse --show-toplevel)"

STACK=""                 # php | go — escolhido no setup
MAIN_BRANCH=main         # so recebe release
DEVELOP_BRANCH=develop   # onde o trabalho se integra
BRANCH_TYPES="feat fix chore refactor perf docs test"
GATE_CMD="make ci"       # o gate: roda UMA vez por arvore
PREPUSH_GATE=1           # 1 = o pre-push roda o gate para arvore nunca gateada
FORMAT_ON_COMMIT=1       # 1 = o pre-commit formata os arquivos staged
SUBJECT_MAX=72           # tamanho maximo do assunto do commit
REFUSE_AI_COAUTHOR=0     # 1 = recusa Co-Authored-By de assistente
DEPLOY_WORKFLOW=""       # arquivo do workflow que o push em main dispara; vazio = nao acompanha
RELEASE_BUILD_WORKFLOW="" # workflow que constroi a release no push de release/vX.Y.Z; o ship espera ele; vazio = nao confere

if [ -f .workflow.conf ]; then
  # shellcheck disable=SC1091
  . ./.workflow.conf
fi

COMMIT_TYPES='feat|fix|perf|refactor|docs|test|build|ci|chore|style'

current_branch() {
  git rev-parse --abbrev-ref HEAD
}

has_branch() {
  git rev-parse --verify --quiet "refs/heads/$1" >/dev/null
}

has_origin() {
  git remote get-url origin >/dev/null 2>&1
}

# Traz o main local para onde o origin esta, antes de qualquer decisao que parta
# dele. Um clone que nunca deu `git pull` em main guarda o main de semanas atras,
# e o develop criado dali nasce sem o que a producao ja roda.
#
# So faz fast-forward. Main local A FRENTE do origin ou divergente e erro, e para
# tudo: main so anda por `make ship`, entao isso nao deveria existir.
pull_main() {
  has_origin || return 0
  git ls-remote --exit-code --heads origin "$MAIN_BRANCH" >/dev/null 2>&1 || return 0

  git fetch --quiet origin "$MAIN_BRANCH"
  has_branch "$MAIN_BRANCH" || { git branch "$MAIN_BRANCH" "origin/$MAIN_BRANCH"; return 0; }

  git merge-base --is-ancestor "origin/$MAIN_BRANCH" "$MAIN_BRANCH" && return 0

  if ! git merge-base --is-ancestor "$MAIN_BRANCH" "origin/$MAIN_BRANCH"; then
    echo "$MAIN_BRANCH local e origin/$MAIN_BRANCH divergiram. Resolva a mao antes de seguir." >&2
    exit 1
  fi

  if [ "$(current_branch)" = "$MAIN_BRANCH" ]; then
    git merge --ff-only --quiet "origin/$MAIN_BRANCH"
  else
    git update-ref "refs/heads/$MAIN_BRANCH" "$(git rev-parse "origin/$MAIN_BRANCH")"
  fi
  echo "$MAIN_BRANCH local atualizado para origin ($(git rev-parse --short "$MAIN_BRANCH"))."
}

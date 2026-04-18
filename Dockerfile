# Dockerfile utilisé par GoReleaser — le binaire est déjà compilé à ce stade.
# GoReleaser copie l'exécutable dans /gtc avant de construire l'image.
FROM alpine:3.21

# git  : nécessaire pour gtc init (détection du remote) et FindGitRoot
# ca-certificates : requises pour les appels HTTPS vers les APIs GitHub/GitLab
# tzdata : fuseaux horaires pour les dates relatives dans gtc watch
RUN apk add --no-cache git ca-certificates tzdata

# Utilisateur non-root par défaut (bonne pratique de sécurité)
RUN addgroup -S gtc && adduser -S gtc -G gtc
USER gtc

COPY gtc /usr/local/bin/gtc

ENTRYPOINT ["gtc"]

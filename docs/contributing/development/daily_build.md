# daily smoke

## build CI image

With cache acceleration, build two ci image and push to ghcr

(1) ****-ci:${ref} : the normal image

(2) ****-ci:${ref}-rate : image who turns on 'go race' and 'deadlock detect'

the CI will clean ci images at interval

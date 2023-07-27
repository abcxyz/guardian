# Copyright 2023 The Authors (see AUTHORS file)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM --platform=$BUILDPLATFORM alpine AS builder

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Normally we would set this to run as "nobody", but to play nicely with GitHub
# Actions, it must run as the default user:
#
#   https://docs.github.com/en/actions/creating-actions/dockerfile-support-for-github-actions#user
#
# USER nobody

COPY guardian /guardian
ENTRYPOINT ["/guardian"]

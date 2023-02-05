#!/bin/bash
c=$(git rev-parse --short HEAD); b=$(git name-rev --name-only "$c"); echo -n "$c ($b branch)"

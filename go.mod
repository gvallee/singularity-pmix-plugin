module github.com/gvallee/singularity-pmix-plugin

go 1.13

require (
	github.com/gvallee/go_util v1.0.0
	github.com/spf13/cobra v0.0.5
	github.com/sylabs/singularity v0.0.0
)

replace github.com/sylabs/singularity => ./singularity_source

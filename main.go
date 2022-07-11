package main

import (
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi-vault/sdk/v4/go/vault"
	"github.com/pulumi/pulumi-vault/sdk/v4/go/vault/generic"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		/*
			Reproduction of pulumi refresh w/ stale/expired credentials issue
			TLDR- refresh of a stack is failing when vault provider contains refresh to stale/expired credentials.
				  as refresh does not execute the Pulumi program prior to attempting a refresh of state, this op will fail
				  when provider(s) are using stale/expired credentials

			Steps to reproduce:
			1. Install vault locally or have access to an existing vault instance
			2. Create simple Pulumi program which creates a vault provider and uses it in some fashion.
			3. Create a vault token (initial token) and provide token to vault provider via environment variable.
				- export VAULT_TOKEN=$(vault token create -ttl=3m -format=json | jq '.auth.client_token')
				- create a vault token with a TTL of 3m and set as env var
			4. Pulumi up your program.
				- program should initially suceed
			5. After 3 minutes have transpired and token has expired...
			6. Create a second (valid) vault token and set the VAULT_TOKEN environment variable.
				- export VAULT_TOKEN=$(vault token create -ttl=3m -format=json | jq '.auth.client_token')
				- create a vault token with a TTL of 3m and set as env var
			7. Pulumi up your program w/ refresh...pulumi up -r
			8. Program should fail.
			9. Pulumi up with new token should succeed...pulumi up
			10. Note: vault provider needs to be updated with a valid token prior to stack deletion.
		*/

		vaultProvider, err := vault.NewProvider(ctx, "vault", &vault.ProviderArgs{
			Token:   pulumi.String(os.Getenv("VAULT_TOKEN")),
			Address: pulumi.String(os.Getenv("VAULT_ADDR")),
		})

		if err != nil {
			return err
		}

		secret, err := generic.NewSecret(ctx, "secret-1", &generic.SecretArgs{
			Path: pulumi.String("secret/test-path"),
			DataJson: pulumi.String(`{
				"key": "value"
			}
			`),
		}, pulumi.Provider(vaultProvider))

		if err != nil {
			return err
		}
		// Create an AWS resource (S3 Bucket)
		bucket, err := s3.NewBucket(ctx, "my-bucket", nil)
		if err != nil {
			return err
		}

		_, err = s3.NewBucketObject(ctx, "path-obj", &s3.BucketObjectArgs{
			Bucket:  bucket.ID(),
			Content: secret.Path,
		})

		if err != nil {
			return err
		}

		// Export the name of the bucket
		ctx.Export("bucketName", bucket.ID())
		return nil
	})
}

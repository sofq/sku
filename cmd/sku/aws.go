package sku

import "github.com/spf13/cobra"

// newAWSCmd returns the `sku aws ...` parent. Leaves are registered
// statically from the §3 shard inventory — m3a.1 adds ec2 and rds;
// later sub-milestones append s3, lambda, ebs, dynamodb, cloudfront.
func newAWSCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "aws",
		Short: "AWS pricing subcommands",
	}
	c.AddCommand(newAWSEC2Cmd())
	c.AddCommand(newAWSRDSCmd())
	c.AddCommand(newAWSAuroraCmd())
	c.AddCommand(newAWSS3Cmd())
	c.AddCommand(newAWSLambdaCmd())
	c.AddCommand(newAWSEBSCmd())
	c.AddCommand(newAWSDynamoDBCmd())
	c.AddCommand(newAWSCloudFrontCmd())
	c.AddCommand(newAWSElastiCacheCmd())
	return c
}

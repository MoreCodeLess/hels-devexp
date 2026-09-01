package scanner

import "sort"

// eventTypeToFlociService mapea el tipo de trigger de una función (la key
// dentro de un item de "events") al servicio de floci que hace falta para
// simularlo localmente.
var eventTypeToFlociService = map[string]string{
	"http":            "apigateway",
	"httpApi":         "apigatewayv2",
	"websocket":       "apigatewayv2",
	"sqs":             "sqs",
	"sns":             "sns",
	"schedule":        "events",
	"s3":              "s3",
	"stream":          "dynamodb",
	"cognitoUserPool": "cognito-idp",
}

// cfnTypeToFlociService mapea el Type de un recurso de CloudFormation al
// servicio de floci correspondiente.
var cfnTypeToFlociService = map[string]string{
	"AWS::SQS::Queue":                    "sqs",
	"AWS::SNS::Topic":                    "sns",
	"AWS::SNS::Subscription":             "sns",
	"AWS::DynamoDB::Table":                "dynamodb",
	"AWS::S3::Bucket":                     "s3",
	"AWS::ElastiCache::CacheCluster":      "elasticache",
	"AWS::ElastiCache::ReplicationGroup":  "elasticache",
	"AWS::RDS::DBInstance":                "rds",
	"AWS::RDS::DBCluster":                 "rds",
	"AWS::ECS::Cluster":                   "ecs",
	"AWS::ECS::Service":                   "ecs",
	"AWS::ECS::TaskDefinition":            "ecs",
	"AWS::ECR::Repository":                "ecr",
	"AWS::KMS::Key":                       "kms",
	"AWS::SSM::Parameter":                 "ssm",
	"AWS::SecretsManager::Secret":         "secretsmanager",
	"AWS::StepFunctions::StateMachine":    "states",
	"AWS::Kinesis::Stream":                "kinesis",
	"AWS::Events::Rule":                   "events",
	"AWS::Events::EventBus":               "events",
	"AWS::Cognito::UserPool":              "cognito-idp",
	"AWS::Cognito::UserPoolClient":        "cognito-idp",
	"AWS::Logs::LogGroup":                 "logs",
}

// DetectAWSServices mira las funciones (por sus events) y los recursos de
// CloudFormation de todos los servicios escaneados y devuelve, ordenada, la
// lista de servicios de floci que hacen falta para simular todo localmente.
// "lambda" siempre se incluye si hay al menos una función declarada.
func DetectAWSServices(services []*ServiceDef) []string {
	set := make(map[string]bool)

	for _, s := range services {
		if len(s.Functions) > 0 {
			set["lambda"] = true
		}
		for _, fn := range s.Functions {
			for _, ev := range fn.Events {
				if svc, ok := eventTypeToFlociService[ev.Type]; ok {
					set[svc] = true
				}
			}
		}
		for _, r := range s.Resources {
			if svc, ok := cfnTypeToFlociService[r.Type]; ok {
				set[svc] = true
			}
		}
	}

	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

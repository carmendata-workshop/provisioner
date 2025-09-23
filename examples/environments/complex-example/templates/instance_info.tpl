Instance Information
===================

Instance Name:    ${instance_name}
Instance ID:      ${instance_id}
Environment:      ${environment_name}
Deployment Time:  ${deployment_time}

Tags:
%{ for key, value in tags ~}
  ${key}: ${value}
%{ endfor ~}

Instance Details:
- This instance is part of the ${environment_name} environment
- Instance ${instance_id} is managed by the provisioner scheduler
- Deployment and destruction follow the configured CRON schedules
- All instances share the same environment lifecycle

Status: Active
Created: ${deployment_time}
Workspace Deployment Information
=====================================

Workspace Name: ${workspace_name}
Workspace Type: ${workspace_type}
Deployment Time:  ${deployment_time}
Instance Count:   ${instance_count}

Tags:
%{ for key, value in tags ~}
  ${key}: ${value}
%{ endfor ~}

Deployment Details:
- This workspace was deployed by the OpenTofu Workspace Scheduler
- Check the provisioner logs for deployment status
- Configuration can be modified in config.json
- Schedule changes are automatically detected and applied

Next Steps:
1. Verify all instance files were created
2. Check deployment logs for any issues
3. Update schedules in config.json as needed
4. Monitor workspace through provisioner service

Generated at: ${deployment_time}
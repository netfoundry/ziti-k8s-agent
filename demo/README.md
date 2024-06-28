## Demo steps

### Use Case Target

Cloud Native Aplications that are distributed over more than one region and are required to enforce granular access controls ensuring that only authorized users/microservices can interact with specific microservices at the pod level.

![image](./images/k8s-distributed-app.svg)

### Prerequisities:
Following binaries to be installed in the environment. 
1. [ziti cli](https://github.com/openziti/ziti/releases)
1. [gcloud cli](https://cloud.google.com/sdk/docs/install)
1. [gcloud auth plugin](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl#install_plugin)
1. [eksctl cli](https://eksctl.io/installation/)
1. [aws cli](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html#getting-started-install-instructions)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
1. [postman cli](https://learning.postman.com/docs/postman-cli/postman-cli-installation/)
1. [jq](https://jqlang.github.io/jq/download/)

## NetFoundry Components

### Create Network and Public Router

1. Login to  [NetFoundry Console](https://cloudziti.io/)
1. Create Network
1. Create Public Router with role attribute == `public`

### Create Admin User:

1. Identities --> Create

1. Fill in details and save (i.e. ott type)

    ![image](./images/CreateAdminIdentity.png)

1. Download jwt token and enroll it.

    ![image](./images/EnrollAdminIdentity.png)
    ```shell
    ziti edge enroll -j adminUser.jwt -o adminUser.json
    ```

### Create Test User:

1. Repeat the same steps but dont enable `IS Admin` option

    ![image](./images/CreateTestIdentity.png)

    If using ziti-edge-tunnel - [Linux based Installations](https://openziti.io/docs/reference/tunnelers/linux/)
    ```shell
    sudo ziti-edge-tunnel add --jwt "$(< ./testUser.jwt)" --identity testUser
    ```
    If using Windows/Mac App - [WinOS Enrolling](https://openziti.io/docs/reference/tunnelers/windows#enrolling), [MacOS Enrolling](https://openziti.io/docs/reference/tunnelers/windows#enrolling)


### Export NetFoundry Network and EKS/GKE Details

--------------------

**IMPORTANT: Copy the code directly to the linux terminal to create required files/resources. In AWS, the VPC and network will be created part of `eksctl create cluster` command and one needs to have administrator permissions. Whereas in GKE, it is expected that VPC and network are already prebuilt. The service account is the part before @ and can be found under IAM-->Permissions, i.e. `{GKE_SERVICE_ACCOUNT}@{GKE_PROJECT_NAME}.iam.gserviceaccount.com`. The subnetwork is the subnet name and must be in the same region as indicated in GKE_REGION. If you already have clusters up, then you can skip to [Export Cluster Context Names](#export-cluster-context-names) section**

--------------------

```shell
export NF_IDENTITY_PATH="path/to/adminUser.json"
export CLUSTER_NAME=""
export AWS_PROFILE=""
export AWS_SSO_ACCOUNT_ID=""
export AWS_SSO_SESSION=""
export AWS_SSO_START_URL=""
export AWS_REGION=""
export GKE_PROJECT_NAME=""
export GKE_NETWORK_NAME=""
export GKE_SUBNETWORK_NAME=""
export GKE_SERVICE_ACCOUNT=""
export GKE_REGION=""
```

### Create Services, Service Policies, Edge Router Policy, Service Edge Router Policy
1. Get ctrl-address/cert/ca/key files created.
    ```shell
    export CTRL_ADDRESS=$(sed "s/client/management/" <<< `jq -r .ztAPI $NF_IDENTITY_PATH`)
    export NF_IDENTITY_CERT_PATH="nf_identity_cert.pem"
    export NF_IDENTITY_KEY_PATH="nf_identity_key.pem"
    export NF_IDENTITY_CA_PATH="nf_identity_ca.pem"
    sed "s/pem://" <<< `jq -r .id.cert $NF_IDENTITY_PATH` > $NF_IDENTITY_CERT_PATH
    sed "s/pem://" <<< `jq -r .id.key $NF_IDENTITY_PATH` > $NF_IDENTITY_KEY_PATH
    sed "s/pem://" <<< `jq -r .id.ca $NF_IDENTITY_PATH` > $NF_IDENTITY_CA_PATH
    export NF_IDENTITY_CERT=$(sed "s/pem://" <<< `jq .id.cert $NF_IDENTITY_PATH`)
    export NF_IDENTITY_KEY=$(sed "s/pem://" <<< `jq .id.key $NF_IDENTITY_PATH`)
    export NF_IDENTITY_CA=$(sed "s/pem://" <<< `jq .id.ca $NF_IDENTITY_PATH`)
    ```

1. Copy the code into a terminal to create Postman collection file

    <details><summary>Code</summary><p>

    ```shell
    cat <<EOF >Istio_Bookinfo_App.postman_collection.json
    {
      "info": {
        "_postman_id": "843b4884-994c-4611-9db1-3c63ea78e904",
        "name": "Istio Bookinfo App",
        "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        "_exporter_id": "3145648"
      },
      "item": [
        {
          "name": "Authenticate",
          "event": [
            {
              "listen": "test",
              "script": {
                "exec": [
                  "pm.globals.set(\"api_token\", pm.response.json().data.token);"
                ],
                "type": "text/javascript",
                "packages": {}
              }
            },
            {
              "listen": "prerequest",
              "script": {
                "exec": [
                  ""
                ],
                "type": "text/javascript",
                "packages": {}
              }
            }
          ],
          "request": {
            "method": "POST",
            "header": [],
            "body": {
              "mode": "raw",
              "raw": "{}",
              "options": {
                "raw": {
                  "language": "json"
                }
              }
            },
            "url": {
              "raw": "{{controller-api-endpoint}}/authenticate?method=cert",
              "host": [
                "{{controller-api-endpoint}}"
              ],
              "path": [
                "authenticate"
              ],
              "query": [
                {
                  "key": "method",
                  "value": "cert"
                }
              ]
            }
          },
          "response": []
        },
        {
          "name": "Configs",
          "item": [
            {
              "name": "Post Config Details Host",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('hostConfigId1', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n    \"name\": \"details.host.v1\",\r\n    \"configTypeId\": \"NH5p4FpGR\",\r\n    \"data\": {\r\n        \"address\": \"127.0.0.1\",\r\n        \"allowedPortRanges\": [\r\n            {\r\n                \"high\": 9080,\r\n                \"low\": 9080\r\n            }\r\n        ],\r\n        \"allowedProtocols\": [\r\n            \"tcp\"\r\n        ],\r\n        \"forwardPort\": true,\r\n        \"forwardProtocol\": true,\r\n        \"listenOptions\": {\r\n            \"bindUsingEdgeIdentity\": false,\r\n            \"connectTimeout\": \"1s\",\r\n            \"connectTimeoutSeconds\": 1,\r\n            \"identity\": \"\",\r\n            \"precedence\": \"default\"\r\n        }\r\n    }\r\n}"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Details Intercept",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('interceptConfigId1', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"details.intercept.v1\",\r\n  \"configTypeId\": \"g7cIWbcGg\",\r\n  \"data\":{\r\n    \"addresses\":[\r\n      \"details\"],\r\n    \"dialOptions\":{\r\n      \"identity\":\"\"\r\n    },\r\n    \"portRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"protocols\":[\r\n      \"tcp\"],\r\n    \"sourceIp\":\"\"\r\n  }\r\n}\r\n\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Productpage Host",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('hostConfigId2', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"productpage.host.v1\",\r\n  \"configTypeId\": \"NH5p4FpGR\",\r\n  \"data\":{\r\n    \"address\":\"127.0.0.1\",\r\n    \"allowedPortRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"allowedProtocols\":[\r\n      \"tcp\"],\r\n    \"forwardPort\":true,\r\n    \"forwardProtocol\":true,\r\n    \"listenOptions\":{\r\n      \"bindUsingEdgeIdentity\":false,\r\n      \"connectTimeout\":\"1s\",\r\n      \"connectTimeoutSeconds\":1,\r\n      \"identity\":\"\",\r\n      \"precedence\":\"default\"\r\n    }\r\n  }\r\n}\r\n\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Productpage Intercept",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('interceptConfigId2', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"productpage.intercept.v1\",\r\n  \"configTypeId\": \"g7cIWbcGg\",\r\n  \"data\":{\r\n    \"addresses\":[\r\n      \"productpage.ziti\"],\r\n    \"dialOptions\":{\r\n      \"identity\":\"\"\r\n    },\r\n    \"portRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"protocols\":[\r\n      \"tcp\"],\r\n    \"sourceIp\":\"\"\r\n  }\r\n}\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Ratings Host",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('hostConfigId3', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"ratings.host.v1\",\r\n  \"configTypeId\": \"NH5p4FpGR\",\r\n  \"data\":{\r\n    \"address\":\"127.0.0.1\",\r\n    \"allowedPortRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"allowedProtocols\":[\r\n      \"tcp\"],\r\n    \"forwardPort\":true,\r\n    \"forwardProtocol\":true,\r\n    \"listenOptions\":{\r\n      \"bindUsingEdgeIdentity\":false,\r\n      \"connectTimeout\":\"1s\",\r\n      \"connectTimeoutSeconds\":1,\r\n      \"identity\":\"\",\r\n      \"precedence\":\"default\"\r\n    }\r\n  }\r\n}\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Ratings Intercept",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('interceptConfigId3', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"ratings.intercept.v1\",\r\n  \"configTypeId\": \"g7cIWbcGg\",\r\n  \"data\":{\r\n    \"addresses\":[\r\n      \"ratings\"],\r\n    \"dialOptions\":{\r\n      \"identity\":\"\"\r\n    },\r\n    \"portRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"protocols\":[\r\n      \"tcp\"],\r\n    \"sourceIp\":\"\"\r\n  }\r\n}\r\n\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Reviews Host",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('hostConfigId4', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"reviews.host.v1\",\r\n  \"configTypeId\": \"NH5p4FpGR\",\r\n  \"data\":{\r\n    \"address\":\"127.0.0.1\",\r\n    \"allowedPortRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"allowedProtocols\":[\r\n      \"tcp\"],\r\n    \"forwardPort\":true,\r\n    \"forwardProtocol\":true,\r\n    \"listenOptions\":{\r\n      \"bindUsingEdgeIdentity\":false,\r\n      \"connectTimeout\":\"1s\",\r\n      \"connectTimeoutSeconds\":1,\r\n      \"identity\":\"\",\r\n      \"precedence\":\"default\"\r\n    }\r\n  }\r\n}\r\n\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Post Config Reviews Intercept",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      "const jsonData = pm.response.json();\r",
                      "postman.setEnvironmentVariable('interceptConfigId4', jsonData.data.id)"
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\r\n  \"name\":\"reviews.intercept.v1\",\r\n  \"configTypeId\": \"g7cIWbcGg\",\r\n  \"data\":{\r\n    \"addresses\":[\r\n      \"reviews\"],\r\n    \"dialOptions\":{\r\n      \"identity\":\"\"\r\n    },\r\n    \"portRanges\":[\r\n      {\r\n        \"high\":9080,\r\n        \"low\":9080\r\n      }],\r\n    \"protocols\":[\r\n      \"tcp\"],\r\n    \"sourceIp\":\"\"\r\n  }\r\n}\r\n\r\n\r\n\r\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/configs/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "configs",
                    ""
                  ]
                }
              },
              "response": []
            }
          ]
        },
        {
          "name": "Services",
          "item": [
            {
              "name": "Create Service Details",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\": \"details-service\",\n  \"roleAttributes\": [\n    \"details\"\n  ],\n  \"configs\": [\n    \"{{hostConfigId1}}\",\n    \"{{interceptConfigId1}}\"\n  ],\n  \"encryptionRequired\": true,\n  \"terminatorStrategy\": \"random\",\n  \"tags\": {}\n}"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/services/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "services",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service Productpage",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\": \"productpage-service\",\n  \"roleAttributes\": [\n    \"productpage\"\n  ],\n  \"configs\": [\n    \"{{hostConfigId2}}\",\n    \"{{interceptConfigId2}}\"\n  ],\n  \"encryptionRequired\": true,\n  \"terminatorStrategy\": \"random\",\n  \"tags\": {}\n}"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/services/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "services",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service Ratings",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\": \"ratings-service\",\n  \"roleAttributes\": [\n    \"ratings\"\n  ],\n  \"configs\": [\n    \"{{hostConfigId3}}\",\n    \"{{interceptConfigId3}}\"\n  ],\n  \"encryptionRequired\": true,\n  \"terminatorStrategy\": \"smartrouting\",\n  \"tags\": {}\n}"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/services/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "services",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service Reviews",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\": \"reviews-service\",\n  \"roleAttributes\": [\n    \"reviews\"\n  ],\n  \"configs\": [\n    \"{{hostConfigId4}}\",\n    \"{{interceptConfigId4}}\"\n  ],\n  \"encryptionRequired\": true,\n  \"terminatorStrategy\": \"random\",\n  \"tags\": {}\n}"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/services/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "services",
                    ""
                  ]
                }
              },
              "response": []
            }
          ]
        },
        {
          "name": "Service-Policies",
          "item": [
            {
              "name": "Create Service-Policy App User Dial",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"app-user-service-policy-dial\",\n  \"type\":\"Dial\",\n  \"serviceRoles\":[\n    \"#productpage\"],\n  \"identityRoles\":[\n    \"#users\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Productpage Bind",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"productpage-service-policy-bind\",\n  \"type\":\"Bind\",\n  \"serviceRoles\":[\n    \"#productpage\"],\n  \"identityRoles\":[\n    \"#productpage\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Productpage Dial",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"productpage-service-policy-dial\",\n  \"type\":\"Dial\",\n  \"serviceRoles\":[\n    \"#details\",\n    \"#reviews\"],\n  \"identityRoles\":[\n    \"#productpage\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Details Bind",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"details-service-policy-bind\",\n  \"type\":\"Bind\",\n  \"serviceRoles\":[\n    \"#details\"],\n  \"identityRoles\":[\n    \"#details\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Reviews Bind",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"reviews-service-policy-bind\",\n  \"type\":\"Bind\",\n  \"serviceRoles\":[\n    \"#reviews\"],\n  \"identityRoles\":[\n    \"#reviews\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Reviews Dial",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"reviews-service-policy-dial\",\n  \"type\":\"Dial\",\n  \"serviceRoles\":[\n    \"#ratings\"],\n  \"identityRoles\":[\n    \"#reviews\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            },
            {
              "name": "Create Service-Policy Ratings Bind",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"ratings-service-policy-bind\",\n  \"type\":\"Bind\",\n  \"serviceRoles\":[\n    \"#ratings\"],\n  \"identityRoles\":[\n    \"#ratings\"],\n  \"postureCheckRoles\":[\n  ],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-policies/",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-policies",
                    ""
                  ]
                }
              },
              "response": []
            }
          ]
        },
        {
          "name": "Service-Edge-Router-Policies",
          "item": [
            {
              "name": "Create service-edge-router-policy",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"public-service-router-policy\",\n  \"serviceRoles\":[\n    \"#details\",\n    \"#productpage\",\n    \"#ratings\",\n    \"#reviews\"],\n  \"edgeRouterRoles\":[\n    \"#public\"],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/service-edge-router-policies",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "service-edge-router-policies"
                  ]
                }
              },
              "response": []
            }
          ]
        },
        {
          "name": "Edge-Router-Policies",
          "item": [
            {
              "name": "Create edge-router-policy",
              "event": [
                {
                  "listen": "test",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                },
                {
                  "listen": "prerequest",
                  "script": {
                    "exec": [
                      ""
                    ],
                    "type": "text/javascript",
                    "packages": {}
                  }
                }
              ],
              "request": {
                "method": "POST",
                "header": [
                  {
                    "key": "Content-Type",
                    "value": "application/json"
                  },
                  {
                    "key": "zt-session",
                    "value": "{{api_token}}"
                  }
                ],
                "body": {
                  "mode": "raw",
                  "raw": "{\n  \"name\":\"public-router-policy\",\n  \"edgeRouterRoles\":[\n    \"#public\"],\n  \"identityRoles\":[\n    \"#details\",\n    \"#users\",\n    \"#productpage\",\n    \"#ratings\",\n    \"#reviews\"],\n  \"semantic\":\"AnyOf\",\n  \"tags\":{\n  }\n}\n"
                },
                "url": {
                  "raw": "{{controller-api-endpoint}}/edge-router-policies",
                  "host": [
                    "{{controller-api-endpoint}}"
                  ],
                  "path": [
                    "edge-router-policies"
                  ]
                }
              },
              "response": []
            }
          ]
        }
      ]
    }
    EOF
    ```

    </p></details>

1. Copy the code into a terminal to create Postman collection environmental variables file

    <details><summary>Code</summary><p>

    ```shell
    cat <<EOF >Istio_Bookinfo_App.postman_environment.json
      {
        "id": "8fd7318e-55d4-4814-b035-2b1c0d6edd28",
        "name": "Istio Bookinfo App",
        "values": [
          {
            "key": "controller-api-endpoint",
            "value": "$CTRL_ADDRESS",
            "enabled": true
          },
          {
            "key": "hostConfigId1",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "interceptConfigId1",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "hostConfigId2",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "interceptConfigId2",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "hostConfigId3                                                                   ",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "interceptConfigId3",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "hostConfigId4",
            "value": "",
            "type": "default",
            "enabled": true
          },
          {
            "key": "interceptConfigId4",
            "value": "",
            "type": "default",
            "enabled": true
          }
        ],
        "_postman_variable_scope": "environment",
        "_postman_exported_at": "2024-06-15T01:59:34.264Z",
        "_postman_exported_using": "Postman/11.2.1"
      }
    EOF
    ```

    </p></details>

1. Run postman cli to configure NetFoundry Components.
    ```shell
    postman collection run Istio_Bookinfo_App.postman_collection.json \
            -e Istio_Bookinfo_App.postman_environment.json \
            --ssl-client-cert $NF_IDENTITY_CERT_PATH \
            --ssl-client-key $NF_IDENTITY_KEY_PATH \
            --ssl-extra-ca-certs $NF_IDENTITY_CA_PATH
    ```

## Create Cluster(s)

### AWS

1. Create AWS Profiles if not done already
    
    ***Note: May have to create ~/.aws folder first.***

    <details><summary>Code</summary><p>

    ```shell
    cat <<EOF >~/.aws/config
      [sso-session ${AWS_SSO_SESSION}]
      sso_start_url = ${AWS_SSO_START_URL}
      sso_region = us-east-1
      sso_registration_scopes = sso:account:access
      [profile ${AWS_PROFILE}]
      sso_session = ${AWS_SSO_SESSION}
      sso_account_id = ${AWS_SSO_ACCOUNT_ID}
      sso_role_name = Administrator
      region = us-east-2
      output = json
      [default]
      region = us-east-2
    EOF
    ```

    </p></details>

1. Login with SSO
    ```shell
    aws sso login --profile $AWS_PROFILE
    ```
    If can not launch browser from terminal
    ```shell
    aws sso login --profile $AWS_PROFILE --no-browser
    ```
1. Create cluster config template

    <details><summary>Code</summary><p>

    ```shell
    cat <<EOF >eks-cluster.yaml
    apiVersion: eksctl.io/v1alpha5
    kind: ClusterConfig
    metadata:
      name: ${CLUSTER_NAME}
      region: ${AWS_REGION}
      version: "1.28"
    managedNodeGroups:
    - name: ng-1
      instanceType: t3.medium
      iam:
          withAddonPolicies:
            ebs: true
            fsx: true
            efs: true
      desiredCapacity: 2
      privateNetworking: true
      labels:
        nodegroup-type: workloads
      tags:
        nodegroup-role: worker
    vpc:
      cidr: 10.10.0.0/16
      publicAccessCIDRs: []
      # disable public access to endpoint and only allow private access
      clusterEndpoints:
        publicAccess: true
        privateAccess: true
    EOF
    ```

    </p></details>

1. Create cluster
```shell
eksctl create cluster -f ./eks-cluster.yaml --profile $AWS_PROFILE
```

### GCLOUD
1. Login
    ```shell
    gcloud auth login
    ```
    If can not launch browser from terminal
    ```shell
    gcloud auth login --no-browser
    ````
1. Create cluster
    ```shell
    gcloud container --project $GKE_PROJECT_NAME clusters create $CLUSTER_NAME \
      --region $GKE_REGION --no-enable-basic-auth \
      --release-channel "regular" --machine-type "e2-medium" \
      --image-type "COS_CONTAINERD" --disk-type "pd-balanced" \
      --disk-size "100" --metadata disable-legacy-endpoints=true \
      --service-account "$GKE_SERVICE_ACCOUNT@$GKE_PROJECT_NAME.iam.gserviceaccount.com" \
      --logging=SYSTEM,WORKLOAD --monitoring=SYSTEM --enable-ip-alias \
      --network "projects/$GKE_PROJECT_NAME/global/networks/$GKE_NETWORK_NAME" \
      --subnetwork "projects/$GKE_PROJECT_NAME/regions/$GKE_REGION/subnetworks/$GKE_SUBNETWORK_NAME" \
      --no-enable-intra-node-visibility --cluster-dns=clouddns --cluster-dns-scope=cluster \
      --default-max-pods-per-node "110" --security-posture=standard \
      --workload-vulnerability-scanning=disabled --no-enable-master-authorized-networks \
      --addons HorizontalPodAutoscaling,NodeLocalDNS,GcePersistentDiskCsiDriver \
      --enable-autoupgrade --enable-autorepair --max-surge-upgrade 1 \
      --max-unavailable-upgrade 0 --binauthz-evaluation-mode=DISABLED \
      --enable-managed-prometheus --enable-shielded-nodes --num-nodes "1"
    ```

### Export Cluster Context Names
If you have your own clusters, then you need to replace the dynamic cluster name search to actual cluster names, i.e. `export AWS_CLUSTER={your cluster namne}`, etc.
```shell
export AWS_CLUSTER=`kubectl config get-contexts -o name | grep $CLUSTER_NAME | grep eksctl`
export GKE_CLUSTER=`kubectl config get-contexts -o name | grep $CLUSTER_NAME | grep gke`
```

### Create Webhook Sidecar Injector Template.

<details><summary>Code</summary><p>

```shell
cat <<EOF >sidecar-injection-webhook-spec.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: ziti
---
apiVersion: v1
kind: Service
metadata:
  name: ziti-sidecar-injector-service
  namespace: ziti
spec:
  selector:
    app: ziti-sidecar-injector-webhook
  ports:
    - name: https
      protocol: TCP
      port: 443
      targetPort: 9443
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ziti-sidecar-injector-wh-deployment
  namespace: ziti
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ziti-sidecar-injector-webhook
  template:
    metadata:
      labels:
        app: ziti-sidecar-injector-webhook
    spec:
      containers:
      - name: ziti-sidecar-injector
        image: docker.io/elblag91/ziti-agent-wh:0.3.3
        imagePullPolicy: Always
        ports:
        - containerPort: 9443
        args:
          - webhook
          - --tls-cert-file
          - /home/ziggy/cert.pem
          - --tls-private-key-file 
          - /home/ziggy/key.pem
        env:
          - name: ZITI_CTRL_ADDRESS
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  address
          - name: ZITI_CTRL_ADMIN_CERT
            valueFrom:
              secretKeyRef:
                name: ziti-ctrl-tls
                key:  tls.crt
          - name: ZITI_CTRL_ADMIN_KEY
            valueFrom:
              secretKeyRef:
                name: ziti-ctrl-tls
                key:  tls.key
          - name: ZITI_ROLE_KEY
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  zitiRoleKey
          - name: POD_SECURITY_CONTEXT_OVERRIDE
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  podSecurityContextOverride
          - name: SEARCH_DOMAIN_LIST
            valueFrom:
              configMapKeyRef:
                name: ziti-ctrl-cfg
                key:  SearchDomainList
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: ziti-tunnel-sidecar
  annotations:
    cert-manager.io/inject-ca-from: ziti/ziti-sidecar-injector-cert
webhooks:
  - name: tunnel.ziti.webhook
    admissionReviewVersions: ["v1"]
    namespaceSelector:
      matchLabels:
        openziti/ziti-tunnel: enabled
    rules:
      - operations: ["CREATE","UPDATE","DELETE"]
        apiGroups: [""]
        apiVersions: ["v1","v1beta1"]
        resources: ["pods"]
        scope: "*"
    clientConfig:
      service:
        name: ziti-sidecar-injector-service
        namespace: ziti
        port: 443
        path: "/ziti-tunnel"
      caBundle: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUdPakNDQkNLZ0F3SUJBZ0lVVHZPVnlqemFBakdnajE2bTA2cHpjcGxBMk40d0RRWUpLb1pJaHZjTkFRRUwKQlFBd2RERUxNQWtHQTFVRUJoTUNWVk14Q3pBSkJnTlZCQWdNQWs1RE1SSXdFQVlEVlFRSERBbERhR0Z5Ykc5MApkR1V4RXpBUkJnTlZCQW9NQ2s1bGRFWnZkVzVrY25reEVUQVBCZ05WQkFzTUNFOXdaVzVhYVhScE1Sd3dHZ1lEClZRUUREQk5hYVhScElGQnZaQ0JEYjI1MGNtOXNiR1Z5TUI0WERUSTBNRFV3TVRFNE1EWXpOVm9YRFRNME1EUXkKT1RFNE1EWXpOVm93ZERFTE1Ba0dBMVVFQmhNQ1ZWTXhDekFKQmdOVkJBZ01BazVETVJJd0VBWURWUVFIREFsRAphR0Z5Ykc5MGRHVXhFekFSQmdOVkJBb01DazVsZEVadmRXNWtjbmt4RVRBUEJnTlZCQXNNQ0U5d1pXNWFhWFJwCk1Sd3dHZ1lEVlFRRERCTmFhWFJwSUZCdlpDQkRiMjUwY205c2JHVnlNSUlDSWpBTkJna3Foa2lHOXcwQkFRRUYKQUFPQ0FnOEFNSUlDQ2dLQ0FnRUFsMURnZGJJMkdLWTl0UU5EOGgxbTBibnlGbVZZclo5am1leUtRcUIreGZiSwpHNEpOcnFtdEdiSmtndUpVOVBaNmxsMTZjam1wUm1ERmp2NDZ0cDhTYWh2alUyeVRPV3dlTmY5WTloZWJmMk84CjkrdzBITXdab0VmUzNWS1VqVXFMcEtGN3lXeVA5ek9icGdoSFc2WHQwQVJFT0s4WXdrTE9BcXlKR2JWNVJjOXYKTlVWOEtnUWwvR1Q0UWs1SklvYitOVk1EenFNUmJVL083dW1sNHVSL3ZOZHVKc1B4dDExbDNjY3YyQTJkZXc2dgpNYXFVcEZzUFVQajUwai9pS1JoSTh5TlYxem9ub1lOUm91QXNJaHN0bWRSOVdLTEE2cVFXQmJPanNCKytpRzNhClp4ZkVBL2V3dW14a0dKV2FKaE1qcjFhNnZieldxUThLK3RzRGMyb29lNUpzcG5OU2ZQakVFc2FZME5Cc0hBdVIKUVdselhNcFdma2ZJc2Erdkc4Tkl0SmFoMW5TUExVMVRpWDR3aWxNSlFadlcrRVhXRnlmUnJBdTZsQ2tLMXdpKwpOaGcwcDhWVFp3djZUclRyQXVWVTlzWDJyV0c3bVN2R1lOdjF6K0dFTk5ISWR2elcwWUQ3ZGRNMThHanlEQ3h0CjZnMUpmZFV6T29uSDZPdFVISXNOYjRtcEJjVC8xOUtENVJxdFoyYXBzdUJ1YTBLUUpKVHpCdkhFd25mUmtRMngKMG8yUU9SWXJiQ2VBMm5LY2Vxc2dITis1RnA3VGN1M24xZ3ZJUTc3K3MxbU41bW1nRGZaQ1dWRDRPOGFNbUY5OQp0a2plU1dlejk5bDhjdUdvUXo3cmRDVW03YlVVRWNKNDlLZmdTYnlHL09XRXdvNWpQVG9ONmlIWCtvVXl4V2tDCkF3RUFBYU9Cd3pDQndEQWRCZ05WSFE0RUZnUVVwU0diN1hsa0xTSGNuKzEyZU8zaFFtaEh1Ymt3SHdZRFZSMGoKQkJnd0ZvQVVwU0diN1hsa0xTSGNuKzEyZU8zaFFtaEh1Ymt3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekJ0QmdOVgpIUkVFWmpCa2dpWjZhWFJwTFhOcFpHVmpZWEl0YVc1cVpXTjBiM0l0YzJWeWRtbGpaUzU2YVhScExuTjJZNEkwCmVtbDBhUzF6YVdSbFkyRnlMV2x1YW1WamRHOXlMWE5sY25acFkyVXVlbWwwYVM1emRtTXVZMngxYzNSbGNpNXMKYjJOaGJJY0Vmd0FBQVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQWdFQWxDRlRzSDlXeGE2YjBINE5YZmhpUVBQRgpCU3lQRlVkZlJZVWNMeGNPOC9VMXQ0S1JEL2NMdDQwcmxhT2ZrRE01dHRoaUpwbHoxRzdPK2U0dndXRk1qRnQ3CjdyUTVVOFdOSi9tUE1UanI0N3BFNUFzUHhGOHR6em1ySHE5S3hKdjFtWjk4WFpiUlQxZlBpeGNTUVFvYlovSFMKcnY1KzVrN1pjcC9SUkR5REtaVy9lbVpwUUxWQW9XdXhkMitVVWpuZHNXc3pxVzZ0a0x2VkFmSnRULzNhY2dBQwozUVBLanMvRG4xNjlXamZRNm8xM05DT0hCcHVTUDluY0pVUytWQmJDQko4QVJiVTFhdG0zSEJHZFpTT0VvWDM2Cmh4bngvM1c0KzZKRW9NbUVMTDFzb0F6dFRPK2NYNWJSRFYrcXlLMXFtQ0ZJb3ZmWFlnT0FBa3BKalhJRG5YSUwKVTJsUFM0enh5Y3JOZytXZVNoWXp5NjZkc0c5WDBndVdiTm9QTDRReHc4bS9ESWFhb3FkTW9WWkorWGlmMHE4agpFQVYwVm9SY3l0c0hZZ2lkNDlVT01uZmhWaDl3TW5ja0tmNGE1ZVAvb2FvcWVBbHZSZlUzZEo3LzErTTZ3SlZyCitwSGZoa3hucEFRZEVZY3FObzU5azlJVGYvTzYxaXZIdTI5UkJTRVVaMUViK3NBQ2E5aUM5TGlvWEEvT3VXcHkKcFV0ejU2bnI1VmZjY0w3MkRRenl0WGQ3SDk0bEdXMno1ZTljaDJoMTNlSkxyZ2JNRDdxMFAySXY4aG5ic05UVwpXd2o5L2lrZURocTBlNCtRRkIrNC9jMWFleGhtckdRZG10OFBkOWVwVUtnMGdLcTlaVHpMbmtxanFnUXUrYlVHCnQxaVEvUXJzVDlLbDBXMjBJRkU9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="
    failurePolicy: Ignore
    sideEffects: None
    timeoutSeconds: 30
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: ziti
  name: ziti-agent-wh-roles
rules:
- apiGroups: [""] # "" indicates the core API group
  resources: ["secrets"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ziti-agent-wh
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ziti-agent-wh-roles
subjects:
- kind: ServiceAccount
  name: default
  namespace: ziti
---
apiVersion: v1
kind: Secret
metadata:
  name: ziti-ctrl-tls
  namespace: ziti
type: kubernetes.io/tls
stringData:
  tls.crt: $NF_IDENTITY_CERT
  tls.key: $NF_IDENTITY_KEY
  tls.ca:  $NF_IDENTITY_CA
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ziti-ctrl-cfg
  namespace: ziti
data:
  address: $CTRL_ADDRESS
  zitiRoleKey: identity.openziti.io/role-attributes
  podSecurityContextOverride: "true"
  SearchDomainList: cluster.local,ziti.svc
EOF
```

</p></details>

### Create Bookinfo App Template

<details><summary>Code</summary><p>

```shell
cat <<EOF >bookinfo-app.yaml
# Copyright Istio Authors
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bookinfo-details
  labels:
    account: details
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: details-v1
  labels:
    app: details
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: details
      version: v1
  template:
    metadata:
      labels:
        app: details
        version: v1
    spec:
      serviceAccountName: bookinfo-details
      containers:
      - name: details
        image: docker.io/istio/examples-bookinfo-details-v1:1.19.1
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 9080
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bookinfo-ratings
  labels:
    account: ratings
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ratings-v1
  labels:
    app: ratings
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ratings
      version: v1
  template:
    metadata:
      labels:
        app: ratings
        version: v1
    spec:
      serviceAccountName: bookinfo-ratings
      containers:
      - name: ratings
        image: docker.io/istio/examples-bookinfo-ratings-v1:1.19.1
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 9080
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bookinfo-reviews
  labels:
    account: reviews
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews-v1
  labels:
    app: reviews
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reviews
      version: v1
  template:
    metadata:
      labels:
        app: reviews
        version: v1
    spec:
      serviceAccountName: bookinfo-reviews
      containers:
      - name: reviews
        image: docker.io/istio/examples-bookinfo-reviews-v1:1.19.1
        imagePullPolicy: IfNotPresent
        env:
        - name: LOG_DIR
          value: "/tmp/logs"
        ports:
        - containerPort: 9080
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: wlp-output
          mountPath: /opt/ibm/wlp/output
      volumes:
      - name: wlp-output
        emptyDir: {}
      - name: tmp
        emptyDir: {}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews-v2
  labels:
    app: reviews
    version: v2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reviews
      version: v2
  template:
    metadata:
      labels:
        app: reviews
        version: v2
    spec:
      serviceAccountName: bookinfo-reviews
      containers:
      - name: reviews
        image: docker.io/istio/examples-bookinfo-reviews-v2:1.19.1
        imagePullPolicy: IfNotPresent
        env:
        - name: LOG_DIR
          value: "/tmp/logs"
        ports:
        - containerPort: 9080
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: wlp-output
          mountPath: /opt/ibm/wlp/output
      volumes:
      - name: wlp-output
        emptyDir: {}
      - name: tmp
        emptyDir: {}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reviews-v3
  labels:
    app: reviews
    version: v3
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reviews
      version: v3
  template:
    metadata:
      labels:
        app: reviews
        version: v3
    spec:
      serviceAccountName: bookinfo-reviews
      containers:
      - name: reviews
        image: docker.io/istio/examples-bookinfo-reviews-v3:1.19.1
        imagePullPolicy: IfNotPresent
        env:
        - name: LOG_DIR
          value: "/tmp/logs"
        ports:
        - containerPort: 9080
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: wlp-output
          mountPath: /opt/ibm/wlp/output
      volumes:
      - name: wlp-output
        emptyDir: {}
      - name: tmp
        emptyDir: {}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bookinfo-productpage
  labels:
    account: productpage
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: productpage-v1
  labels:
    app: productpage
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: productpage
      version: v1
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9080"
        prometheus.io/path: "/metrics"
      labels:
        app: productpage
        version: v1
    spec:
      serviceAccountName: bookinfo-productpage
      containers:
      - name: productpage
        image: docker.io/istio/examples-bookinfo-productpage-v1:1.19.1
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 9080
        volumeMounts:
        - name: tmp
          mountPath: /tmp
      volumes:
      - name: tmp
        emptyDir: {}
EOF
```

</p></details>

### Deploy Webhook Sidecar Injector to EKS
```shell 
kubectl apply -f sidecar-injection-webhook-spec.yaml --context $AWS_CLUSTER
```
### Watch logs from the webhook
```shell
kubectl logs `kubectl get pods -n ziti --context  $AWS_CLUSTER -o name | grep injector-wh` -n ziti --context  $AWS_CLUSTER -f
```
### Deploy Bookinfo to EKS
```shell 
kubectl create namespace test1 --context $AWS_CLUSTER
kubectl label namespace test1 openziti/ziti-tunnel=enabled --context $AWS_CLUSTER
kubectl apply -f bookinfo-app.yaml --context $AWS_CLUSTER -n test1
```

### Deploy Webhook Sidecar Injector to GKE
```shell 
kubectl apply -f sidecar-injection-webhook-spec.yaml --context $GKE_CLUSTER
```
### Watch logs from the webhook
```shell
kubectl logs `kubectl get pods -n ziti --context  $GKE_CLUSTER -o name | grep injector-wh` -n ziti --context  $GKE_CLUSTER -f
```
### Deploy Bookinfo to GKE
```shell
kubectl create namespace test2 --context $GKE_CLUSTER
kubectl label namespace test2 openziti/ziti-tunnel=enabled --context $GKE_CLUSTER
kubectl apply -f bookinfo-app.yaml --context $GKE_CLUSTER -n test2
```

### Check Identities Status
Identities should be all green as shown in this screen shot.
![image](./images/identitiesStatus.png)

### App Test and Verification of Access
Look up pod names for Bookinfo App
```shell
kubectl get pods -n test1 --context  $AWS_CLUSTER
kubectl get pods -n test2 --context  $GKE_CLUSTER
```
Bash script to simulate book reviews data filtered by the reviews pod name 
```shell
for i in $(seq 1 20);
do
    curl -s -X GET http://productpage.ziti:9080/productpage?u=test | grep reviews
done
```
### Results
Pods List
```shell
kubectl get pods -n test1 --context  $AWS_CLUSTER
kubectl get pods -n test2 --context  $GKE_CLUSTER
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-cf74bb974-z8h6j       2/2     Running   0          9m23s
productpage-v1-87d54dd59-dl2fl   2/2     Running   0          9m5s
ratings-v1-7c4bbf97db-9vhc4      2/2     Running   0          9m18s
reviews-v1-5fd6d4f8f8-92rg4      2/2     Running   0          9m13s
reviews-v2-6f9b55c5db-r6ncv      2/2     Running   0          9m13s
reviews-v3-7d99fd7978-t7rnv      2/2     Running   0          9m13s
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-cf74bb974-5l65k       2/2     Running   0          8m44s
productpage-v1-87d54dd59-hvvpn   2/2     Running   0          8m43s
ratings-v1-7c4bbf97db-2m9dk      2/2     Running   0          8m58s
reviews-v1-5fd6d4f8f8-nd9f4      2/2     Running   0          8m57s
reviews-v2-6f9b55c5db-h2gv7      2/2     Running   0          8m51s
reviews-v3-7d99fd7978-hjq4l      2/2     Running   0          8m50s
```

Script Output

All Reviews Pods should be hit at least once
```shell
for i in $(seq 1 20);
do
    curl -s -X GET http://productpage.ziti:9080/productpage?u=test | grep reviews
done
        <u>reviews-v1-5fd6d4f8f8-nd9f4</u>
        <u>reviews-v2-6f9b55c5db-h2gv7</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v1-5fd6d4f8f8-nd9f4</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v2-6f9b55c5db-r6ncv</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v1-5fd6d4f8f8-92rg4</u>
        <u>reviews-v1-5fd6d4f8f8-nd9f4</u>
        <u>reviews-v3-7d99fd7978-hjq4l</u>
        <u>reviews-v3-7d99fd7978-t7rnv</u>
        <u>reviews-v2-6f9b55c5db-r6ncv</u>
        <u>reviews-v3-7d99fd7978-t7rnv</u>
        <u>reviews-v1-5fd6d4f8f8-nd9f4</u>
        <u>reviews-v1-5fd6d4f8f8-92rg4</u>
        <u>reviews-v1-5fd6d4f8f8-92rg4</u>
        <u>reviews-v1-5fd6d4f8f8-nd9f4</u>
        <u>reviews-v1-5fd6d4f8f8-92rg4</u>
```

### Delete App and clean up of identities
```shell
kubectl delete -f bookinfo-app.yaml --context $AWS_CLUSTER -n test1
kubectl delete -f bookinfo-app.yaml --context $GKE_CLUSTER -n test2
```
![image](./images/identitiesStatusDelete.png)

### Delete Clusters
```shell
eksctl delete cluster -f ./eks-cluster.yaml --profile $AWS_PROFILE --force --disable-nodegroup-eviction 
gcloud container --project $GKE_PROJECT_NAME clusters delete $CLUSTER_NAME --region $GKE_REGION
```

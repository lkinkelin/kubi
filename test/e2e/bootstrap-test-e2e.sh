#!/bin/bash
# KIND CLUSTER CREATION
kind delete cluster --name test-e2e-kubi
kind create cluster --name test-e2e-kubi --config test/e2e/conf/kind/cluster-kind.yaml

DOCKER_REGISTRY=docker.io

# PULL AND KIND LOAD IMAGES OF HELPER PODS
docker pull $DOCKER_REGISTRY/bitnami/kubectl
docker pull $DOCKER_REGISTRY/alpine/curl
kind load docker-image $DOCKER_REGISTRY/bitnami/kubectl --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/alpine/curl --name test-e2e-kubi


# OPENLDAP PREREQUISITES AND DEPLOY 
helm repo add helm-openldap https://jp-gouin.github.io/helm-openldap/

# PULL AND KIND LOAD IMAGES OF OPENLDAP PODS
for i in $(helm images get helm-openldap/openldap-stack-ha -f test/e2e/conf/openldap/myvalues.yaml  ); do
    docker pull "$i"
    kind load docker-image "$i" --name test-e2e-kubi 
done

# Create configmap containing ldif file
kubectl -n kube-system apply -f test/e2e/conf/ldap/config.yaml
helm upgrade --install openldap helm-openldap/openldap-stack-ha  -f test/e2e/conf/openldap/myvalues.yaml --namespace kube-system
# We wait 30s for Openldap to pop otherwise, Kubi tries to connect to it directly, fails to open a connection and waits for a new reconciliation loop to occur, which makes the fail test, due to 30s timeout (in e2e_test.go file.)
sleep 30

# CHECK THAT OPENLDAP IS DEPLOYED AND HAS GOOD CONF
# VERIFIER 
# <<<<ldapsearch -x -H ldap://openldap.kube-system.svc.cluster.local -b dc=example,dc=org -D "cn=admin,dc=example,dc=org" -w Not@SecurePassw0rd>>>>

# KUBI OPERATOR DEPLOY 
kubectl -n kube-system create secret generic kubi-secret  --from-literal ldap_passwd='Not@SecurePassw0rd'


# kubi-encryption-secret -> la PKI qui signe les tokens 
# kubi -> je crois que c'est le cert d'authent a l'api server 

./scripts/generate_ecdsa_keys.sh
kubectl -n kube-system create secret generic kubi-encryption-secret --from-file=/tmp/kubi/ecdsa/ecdsa-key.pem --from-file=/tmp/kubi/ecdsa/ecdsa-public.pem

# chmod +x scripts/install_cfssl.sh
# ./scripts/install_cfssl.sh
cat <<EOF | cfssl genkey - | cfssljson -bare server
      {
        "hosts": [
        "kubi.devops.managed.kvm",
        "kubi-svc",
        "kubi-svc.kube-system",
        "kubi-svc.kube-system.svc",
        "kubi-svc.kube-system.svc.cluster.local",
        "10.96.0.2"
         ],
       "CN": "system:node:kubi-svc.kube-system.svc.cluster.local",
       "key": {
       "algo": "ecdsa",
       "size": 256
         },
      "names": [
        {
            "O": "system:nodes"
        }
      ]
     }
EOF


cat <<EOF | kubectl create -f -
 apiVersion: certificates.k8s.io/v1
 kind: CertificateSigningRequest
 metadata:
  name: kubi-svc.kube-system
 spec:
   groups:
     - system:authenticated
   request: $(cat server.csr | base64 | tr -d '\n')
   signerName: kubernetes.io/kubelet-serving
   usages:
     - digital signature
     - key encipherment
     - server auth
EOF

# truc de resigner le cert et replace dans le CSR -> faux 

kubectl certificate approve kubi-svc.kube-system
kubectl get csr kubi-svc.kube-system -o jsonpath='{.status.certificate}' | base64 --decode > server.crt
kubectl -n kube-system create secret tls kubi   --key server-key.pem   --cert server.crt
#create le cm kubi-config avec toutes les infos sur l'AD 
kubectl apply -f test/e2e/conf/kubi/configmap.yaml
kubectl apply -f test/e2e/conf/kubi/kube-crds.yml
kubectl apply -f test/e2e/conf/kubi/kube-prerequisites.yml
kubectl apply -f test/e2e/conf/kubi/black-white-list-cm.yaml
kubectl apply -f test/e2e/conf/kubi/kubi-operator-deployment.yaml
kubectl apply -f test/e2e/conf/kubi/kubi-api-and-auth-webhook-deployment.yaml
kubectl apply -f test/e2e/conf/kubi/rbac.yaml
kubectl apply -f test/e2e/conf/kubi/kubi-netpol-config.yaml
kubectl apply -f test/e2e/conf/kubi/services.yaml

# deploy helper pods which will help us do some curl and kubectl commands
kubectl apply -f test/e2e/conf/helper-pods/

ORG=ca-gip goreleaser release --clean --snapshot
kind load docker-image ghcr.io/ca-gip/kubi-operator:$(git rev-parse --short HEAD)-amd64 --name test-e2e-kubi
kind load docker-image ghcr.io/ca-gip/kubi-api:$(git rev-parse --short HEAD)-amd64 --name test-e2e-kubi
kind load docker-image ghcr.io/ca-gip/kubi-webhook:$(git rev-parse --short HEAD)-amd64 --name test-e2e-kubi

kubectl -n kube-system set image deployment/kubi-operator kubi-operator=ghcr.io/ca-gip/kubi-operator:$(git rev-parse --short HEAD)-amd64
kubectl -n kube-system set image deployment/kubi-deployment api=ghcr.io/ca-gip/kubi-api:$(git rev-parse --short HEAD)-amd64
kubectl -n kube-system set image deployment/kubi-deployment webhook=ghcr.io/ca-gip/kubi-webhook:$(git rev-parse --short HEAD)-amd64

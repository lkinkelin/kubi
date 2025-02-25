#!/bin/bash
# KIND CLUSTER CREATION
kind delete cluster --name test-e2e-kubi
kind create cluster --name test-e2e-kubi --config test/e2e/conf/kind/cluster-kind.yaml

DOCKER_REGISTRY=docker-remote.registry.saas.cagip.group.gca

# PULL AND KIND LOAD IMAGES 
docker pull $DOCKER_REGISTRY/jpgouin/openldap:2.6.8-fix
docker pull $DOCKER_REGISTRY/debian:latest
docker pull $DOCKER_REGISTRY/alpine/openssl:latest
docker pull $DOCKER_REGISTRY/tiredofit/self-service-password:5.2.3
docker pull $DOCKER_REGISTRY/osixia/phpldapadmin:0.9.0
docker pull $DOCKER_REGISTRY/cagip/kubi-operator:v1.30.0-beta1


kind load docker-image $DOCKER_REGISTRY/jpgouin/openldap:2.6.8-fix --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/debian:latest --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/alpine/openssl:latest --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/tiredofit/self-service-password:5.2.3 --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/osixia/phpldapadmin:0.9.0 --name test-e2e-kubi
kind load docker-image $DOCKER_REGISTRY/cagip/kubi-operator:v1.30.0-beta1 --name test-e2e-kubi

# OPENLDAP DEPLOY 
# Create configmap containing ldif file
kubectl -n kube-system apply -f test/e2e/conf/ldap/config.yaml
helm repo add helm-openldap https://jp-gouin.github.io/helm-openldap/
helm upgrade --install openldap helm-openldap/openldap-stack-ha  -f test/e2e/conf/ldap/myvalues.yaml --namespace kube-system
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

cat <<EOF | cfssl genkey - | cfssljson -bare server
      {
        "hosts": [
        "kubi.devops.managed.kvm",
        "kubi-svc",
        "kubi-svc.kube-system",
        "kubi-svc.kube-system.svc",
        "kubi-svc.kube-system.svc.cluster.local"
  
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

# truc de resigner le cert et replace dans le CSR -> Bullshit 

kubectl certificate approve kubi-svc.kube-system
kubectl get csr kubi-svc.kube-system -o jsonpath='{.status.certificate}' | base64 --decode > server.crt
kubectl -n kube-system create secret tls kubi   --key server-key.pem   --cert server.crt
#create le cm kubi-config avec toutes les infos sur l'AD 
kubectl apply -f test/e2e/conf/kubi/configmap.yaml
kubectl apply -f test/e2e/conf/kubi/kube-crds.yml
kubectl apply -f test/e2e/conf/kubi/kube-prerequisites.yml
kubectl apply -f test/e2e/conf/kubi/black-white-list-cm.yaml
kubectl apply -f test/e2e/conf/kubi/kubi-operator-deployment.yaml
kubectl apply -f test/e2e/conf/kubi/rbac.yaml

ORG=ca-gip goreleaser release --clean --snapshot
kind load docker-image ghcr.io/ca-gip/kubi-operator:$(git rev-parse --short HEAD)-amd64 --name test-e2e-kubi
kubectl -n kube-system set image deployment/kubi-operator kubi-operator=ghcr.io/ca-gip/kubi-operator:$(git rev-parse --short HEAD)-amd64

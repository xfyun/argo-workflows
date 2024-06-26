VERSION := $(shell ./sdk_version.py)
ifndef VERSION
  $(error "Failed to obtain a valid version for the SDK")
endif

# work dir
WD := $(shell echo "`pwd`/client")

DOCKER = docker run --rm -v $(WD):/wd --workdir /wd
CHOWN = chown -R $(shell id -u):$(shell id -g)

publish: generate
	pip install setuptools twine build
	python -m build --sdist --wheel --outdir client/dist/ client
	twine check client/dist/*
	twine upload client/dist/* -u __token__ -p ${PYPI_API_TOKEN}

generate:
	rm -Rf $(WD)
	mkdir -p $(WD)
	cat ../../api/openapi-spec/swagger.json | \
		sed 's/io.k8s.api.core.v1.//' | \
		sed 's/io.k8s.apimachinery.pkg.apis.meta.v1.//' \
		> $(WD)/swagger.json
	cp ../../LICENSE $(WD)/LICENSE
	$(DOCKER) openapitools/openapi-generator-cli:v5.2.1 \
		generate \
		--input-spec /wd/swagger.json \
		--generator-name python \
		--output /wd \
		--additional-properties packageVersion=${VERSION} \
		--additional-properties packageName="argo_workflows" \
		--additional-properties projectName="argo-workflows" \
		--additional-properties hideGenerationTimestamp=true \
		--remove-operation-id-prefix \
		--model-name-prefix '' \
		--model-name-suffix '' \
		--artifact-id argo-python-client \
		--global-property modelTests=false \
 		--global-property packageName=argo_workflows \
		--generate-alias-as-model
	# https://vsupalov.com/docker-shared-permissions/#set-the-docker-user-when-running-your-container
	$(CHOWN) $(WD) || sudo $(CHOWN) $(WD)

install:
	pip3 install ./client

test:
	../../hack/access-token.sh init
	env ARGO_TOKEN="`../../hack/access-token.sh get`" python3 tests/*.py

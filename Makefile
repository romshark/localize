.PHONY: generate-formula-json go-generate

generate-languages-json:
	docker run --rm $$(docker build -q -f ./internal/pluralform/Dockerfile ./language) > \
		./internal/pluralform/languages.json

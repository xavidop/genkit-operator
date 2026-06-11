## [0.5.0](https://github.com/xavidop/genkit-operator/compare/v0.4.1...v0.5.0) (2026-06-11)

### Features

* add shared_types.go with InlineModelSpec, InlinePrompt, and PromptSource ([576f460](https://github.com/xavidop/genkit-operator/commit/576f460e90f3f32124e0b7111ba8de601ba4d0d0))
* **api:** update FlowSpec with PromptSource and InlineModelSpec fields ([12f839f](https://github.com/xavidop/genkit-operator/commit/12f839f1e80dd5c42011af37d8d2b91290929895))
* handle inline prompts and inline model in FlowReconciler ([b93e748](https://github.com/xavidop/genkit-operator/commit/b93e7487ca6c325bd5e9fe8b391087a373bdbbc6))
* inline model spec and prompts for Flow and FlowSet ([b830502](https://github.com/xavidop/genkit-operator/commit/b830502b98cc921efeba68f376a69256b7cafaaa))
* update flowset controller and watches for PromptSource/pointer ModelRef ([2084af6](https://github.com/xavidop/genkit-operator/commit/2084af678a3f2d0d474ed1a8587c593851590b98))
* update FlowSetFlow to use PromptSource and optional model fields ([288d426](https://github.com/xavidop/genkit-operator/commit/288d426c10bd8cf558b47c23f0ee0b70b1a3f745))
* update samples to show inline model and prompt options ([0fcf95e](https://github.com/xavidop/genkit-operator/commit/0fcf95ee742e3957ee496841580a2b97a9396eea))

### Bug Fixes

* **controller:** remove early-return bug in flowsReferencingPluginConfig for inline modelSpec flows ([0f65c7f](https://github.com/xavidop/genkit-operator/commit/0f65c7f6936031a661ebefdea853da82a16408f5))
* **controller:** watch PluginConfig changes for inline modelSpec flows ([7617811](https://github.com/xavidop/genkit-operator/commit/7617811b1f066e523724b9d09e59f1e8681f75c4))
* correct pluginConfigRef name in flow sample to match pluginconfig sample ([0ec2655](https://github.com/xavidop/genkit-operator/commit/0ec265574ee392d9dfd2f47e4b75db1a29377e18))
* lint ([3da937e](https://github.com/xavidop/genkit-operator/commit/3da937eae4e28cedbb3f9e99e3c1e11b0f79f905))
* update helm chart ([02186df](https://github.com/xavidop/genkit-operator/commit/02186df21aa9ed15e193792c19c740b9b800cb67))
* use Chainguard static base image and set imagePullPolicy for kind ([c1b5261](https://github.com/xavidop/genkit-operator/commit/c1b52618ad9d5cf5669b5048ccbc8cff0c66a898))

## [0.4.1](https://github.com/xavidop/genkit-operator/compare/v0.4.0...v0.4.1) (2026-06-05)

### Bug Fixes

* release and read from secrets ([fab9aca](https://github.com/xavidop/genkit-operator/commit/fab9aca76e41a69ce39bb1cff3add2b08c642818))

## [0.4.0](https://github.com/xavidop/genkit-operator/compare/v0.3.0...v0.4.0) (2026-06-05)

### Features

* fixing installation ([159d96d](https://github.com/xavidop/genkit-operator/commit/159d96dccaf7e7bebd2033cd4fd82be93cf2a5ad))

## [0.3.0](https://github.com/xavidop/genkit-operator/compare/v0.2.3...v0.3.0) (2026-06-02)

### Features

* done TODOs + replace versions ([f414715](https://github.com/xavidop/genkit-operator/commit/f414715f68041d957809b4e29fa3131a455f0606))

## [0.2.3](https://github.com/xavidop/genkit-operator/compare/v0.2.2...v0.2.3) (2026-06-01)

### Bug Fixes

* lint issues ([25633de](https://github.com/xavidop/genkit-operator/commit/25633deaf2549bddcb181109018815fea46e6a3d))

## [0.2.2](https://github.com/xavidop/genkit-operator/compare/v0.2.1...v0.2.2) (2026-06-01)

### Bug Fixes

* support for Azure ([50e68bf](https://github.com/xavidop/genkit-operator/commit/50e68bfce6667f17bed2c8ea054e84f49b44283f))

## [0.2.1](https://github.com/xavidop/genkit-operator/compare/v0.2.0...v0.2.1) (2026-06-01)

### Bug Fixes

* docs ([2b4b629](https://github.com/xavidop/genkit-operator/commit/2b4b6297d829f3ccfaebe99268d694eeb3388356))

## [0.2.0](https://github.com/xavidop/genkit-operator/compare/v0.1.0...v0.2.0) (2026-06-01)

### Features

* docsite + enw release ([668370f](https://github.com/xavidop/genkit-operator/commit/668370fd1ee0e294f238ea8c10ce9c54fc5c1357))

### Bug Fixes

* docs ([1bc9738](https://github.com/xavidop/genkit-operator/commit/1bc97387887dc20785eeb58379eb3ac5bd542ec6))
* docs ([9558eb6](https://github.com/xavidop/genkit-operator/commit/9558eb69dd9bea372bb1712a270f0983efbf86cc))
* docs ([bd67c3a](https://github.com/xavidop/genkit-operator/commit/bd67c3a20ac8709e4a0fa3adc066903331aedadb))
* force push ([349a895](https://github.com/xavidop/genkit-operator/commit/349a8958462cafe9dea061f353a8343eeb1b8fbb))
* release ([643af61](https://github.com/xavidop/genkit-operator/commit/643af61c10bc1b6cd3f725e06d1cd29413618e99))

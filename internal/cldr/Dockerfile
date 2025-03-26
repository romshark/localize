FROM php:8.2-cli

# 1) Install system dependencies needed by Composer (git, zip, unzip)
RUN apt-get update && apt-get install -y \
    git \
    zip \
    unzip \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 2) Download and install Composer
RUN php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');" \
    && php composer-setup.php \
    && rm composer-setup.php

# 3) Create a minimal composer.json
RUN echo '{}' > composer.json

# 4) Require the gettext/languages package
RUN php composer.phar require --no-interaction gettext/languages

# 5) Generate the plural rules JSON
RUN ./vendor/bin/export-plural-rules prettyjson > languages.json

# 6) By default, print languages.json
CMD ["cat", "languages.json"]

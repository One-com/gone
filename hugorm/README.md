# gone/hugorm

Hugorm is a relatively harmless Viper.

## Overview

Hugorm is work-in-progress.

The fundamental motivation for Hugorm is a recognition that the gone/jconf library is hard to extend with further features. With inspiration from github.com/spf13/viper I acknowledged the need for a model where the config is parsed into an intermediate format of map[string]interface{}.

The Viper library also has limitations (like the inability to handle nil values and emptry maps), so instead of waiting for a Viper2 I refactored it all into my preferences. Hugorm is the result.


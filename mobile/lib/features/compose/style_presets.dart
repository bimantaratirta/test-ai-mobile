enum StylePreset { scandinavian, industrial, japandi, cozyWarm, minimalist, none }

extension StylePresetX on StylePreset {
  String get label {
    switch (this) {
      case StylePreset.scandinavian: return 'Scandinavian';
      case StylePreset.industrial:   return 'Industrial';
      case StylePreset.japandi:      return 'Japandi';
      case StylePreset.cozyWarm:     return 'Cozy Warm';
      case StylePreset.minimalist:   return 'Minimalist';
      case StylePreset.none:         return 'No preset';
    }
  }

  String get apiValue {
    switch (this) {
      case StylePreset.scandinavian: return 'scandinavian';
      case StylePreset.industrial:   return 'industrial';
      case StylePreset.japandi:      return 'japandi';
      case StylePreset.cozyWarm:     return 'cozy_warm';
      case StylePreset.minimalist:   return 'minimalist';
      case StylePreset.none:         return '';
    }
  }
}

enum GenMode { realistic, inspirational }

extension GenModeX on GenMode {
  String get label => this == GenMode.realistic ? 'Realistic' : 'Inspirational';
  String get description => this == GenMode.realistic
      ? 'Keep the room\'s structure. Add furniture, lighting, decor.'
      : 'Reimagine the space freely based on the description.';
  String get apiValue => this == GenMode.realistic ? 'realistic' : 'inspirational';
}

import SwiftUI

// MARK: - Design Tokens

enum Tokens {

    // MARK: Colors
    enum Colors {
        /// Dark teal-green, HSL(150°, 54%, 35%)
        static let primary = Color(hue: 150/360, saturation: 0.54, brightness: 0.35)
        static let primaryForeground = Color.white
        static let primaryLight = Color(hue: 150/360, saturation: 0.40, brightness: 0.90)

        static let secondary = Color(white: 0.92)
        static let secondaryForeground = Color(white: 0.07)

        static let muted = Color(white: 0.97)
        static let mutedForeground = Color(white: 0.45)

        static let destructive = Color(hue: 0, saturation: 0.84, brightness: 0.60)
        static let destructiveForeground = Color.white
        static let destructiveLight = Color(hue: 0, saturation: 0.40, brightness: 0.95)

        static let background = Color.white
        static let backgroundSubtle = Color(white: 0.97)

        static let foreground = Color(white: 0.07)
        static let border = Color(white: 0.92)

        static let success = Color(hue: 142/360, saturation: 0.72, brightness: 0.35)
        static let successLight = Color(hue: 142/360, saturation: 0.40, brightness: 0.92)

        static let warning = Color(hue: 38/360, saturation: 0.92, brightness: 0.60)
        static let warningLight = Color(hue: 38/360, saturation: 0.50, brightness: 0.95)
    }

    // MARK: Typography
    enum Typography {
        static let displayFont = "PlusJakartaSans-Bold"
        static let bodyFont = "Inter-Regular"
        static let bodyMediumFont = "Inter-Medium"
        static let bodySemiBoldFont = "Inter-SemiBold"

        static func display(_ size: CGFloat) -> Font {
            Font.custom(displayFont, size: size).bold()
        }

        static func body(_ size: CGFloat, weight: Font.Weight = .regular) -> Font {
            switch weight {
            case .semibold:
                return Font.custom(bodySemiBoldFont, size: size)
            case .medium:
                return Font.custom(bodyMediumFont, size: size)
            default:
                return Font.custom(bodyFont, size: size)
            }
        }

        // Fallback system fonts for simulator/early testing
        static let displayFallback: Font = .system(size: 48, weight: .bold, design: .default)
        static let h1: Font = .system(size: 32, weight: .bold)
        static let h2: Font = .system(size: 24, weight: .bold)
        static let h3: Font = .system(size: 20, weight: .semibold)
        static let bodyLarge: Font = .system(size: 17, weight: .regular)
        static let bodyBase: Font = .system(size: 15, weight: .regular)
        static let bodySmall: Font = .system(size: 13, weight: .regular)
        static let caption: Font = .system(size: 11, weight: .regular)
        static let label: Font = .system(size: 13, weight: .medium)
    }

    // MARK: Spacing (4pt base grid)
    enum Spacing {
        static let xs: CGFloat = 4
        static let sm: CGFloat = 8
        static let md: CGFloat = 12
        static let base: CGFloat = 16
        static let lg: CGFloat = 20
        static let xl: CGFloat = 24
        static let xxl: CGFloat = 32
        static let xxxl: CGFloat = 48
    }

    // MARK: Radius
    enum Radius {
        static let sm: CGFloat = 8
        static let md: CGFloat = 12
        static let base: CGFloat = 16
        static let lg: CGFloat = 20
        static let full: CGFloat = 9999
    }

    // MARK: Shadows
    enum Shadow {
        static let sm: (color: Color, radius: CGFloat, x: CGFloat, y: CGFloat) =
            (Color.black.opacity(0.06), 4, 0, 2)
        static let base: (color: Color, radius: CGFloat, x: CGFloat, y: CGFloat) =
            (Color.black.opacity(0.08), 8, 0, 4)
        static let lg: (color: Color, radius: CGFloat, x: CGFloat, y: CGFloat) =
            (Color.black.opacity(0.12), 16, 0, 8)
    }
}

// MARK: - Convenience Extensions

extension View {
    func cardShadow() -> some View {
        self.shadow(
            color: Tokens.Shadow.sm.color,
            radius: Tokens.Shadow.sm.radius,
            x: Tokens.Shadow.sm.x,
            y: Tokens.Shadow.sm.y
        )
    }
}

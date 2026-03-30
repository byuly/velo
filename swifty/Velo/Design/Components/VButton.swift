import SwiftUI

// MARK: - Button Variants

enum VButtonVariant {
    case primary, secondary, destructive, ghost, link
}

enum VButtonSize {
    case sm, md, lg, icon

    var height: CGFloat {
        switch self {
        case .sm: return 36
        case .md: return 44
        case .lg: return 52
        case .icon: return 44
        }
    }

    var horizontalPadding: CGFloat {
        switch self {
        case .sm: return 12
        case .md: return 16
        case .lg: return 20
        case .icon: return 12
        }
    }

    var fontSize: CGFloat {
        switch self {
        case .sm: return 13
        case .md: return 15
        case .lg: return 17
        case .icon: return 15
        }
    }
}

// MARK: - VButton

struct VButton: View {
    let title: String
    var icon: String? = nil
    var variant: VButtonVariant = .primary
    var size: VButtonSize = .md
    var isFullWidth: Bool = false
    var isLoading: Bool = false
    var isDisabled: Bool = false
    let action: () -> Void

    private var background: Color {
        switch variant {
        case .primary: return Tokens.Colors.primary
        case .secondary: return Tokens.Colors.secondary
        case .destructive: return Tokens.Colors.destructive
        case .ghost, .link: return .clear
        }
    }

    private var foreground: Color {
        switch variant {
        case .primary: return Tokens.Colors.primaryForeground
        case .secondary: return Tokens.Colors.secondaryForeground
        case .destructive: return Tokens.Colors.destructiveForeground
        case .ghost: return Tokens.Colors.primary
        case .link: return Tokens.Colors.primary
        }
    }

    private var borderColor: Color {
        switch variant {
        case .ghost: return Tokens.Colors.border
        default: return .clear
        }
    }

    var body: some View {
        Button(action: action) {
            HStack(spacing: Tokens.Spacing.sm) {
                if isLoading {
                    ProgressView()
                        .progressViewStyle(.circular)
                        .tint(foreground)
                        .scaleEffect(0.8)
                } else {
                    if let icon {
                        Image(systemName: icon)
                            .font(.system(size: size.fontSize, weight: .medium))
                    }
                    if !title.isEmpty {
                        Text(title)
                            .font(.system(size: size.fontSize, weight: .semibold))
                            .underline(variant == .link)
                    }
                }
            }
            .foregroundStyle(foreground)
            .frame(height: size.height)
            .frame(maxWidth: isFullWidth ? .infinity : nil)
            .padding(.horizontal, variant == .link ? 0 : size.horizontalPadding)
            .background(background)
            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.base))
            .overlay(
                RoundedRectangle(cornerRadius: Tokens.Radius.base)
                    .stroke(borderColor, lineWidth: 1)
            )
            .opacity(isDisabled ? 0.5 : 1.0)
        }
        .disabled(isDisabled || isLoading)
        .buttonStyle(.plain)
    }
}

// MARK: - Icon-only VButton

struct VIconButton: View {
    let icon: String
    var variant: VButtonVariant = .ghost
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: icon)
                .font(.system(size: 16, weight: .medium))
                .foregroundStyle(variant == .ghost ? Tokens.Colors.mutedForeground : Tokens.Colors.primary)
                .frame(width: 40, height: 40)
                .background(Tokens.Colors.muted)
                .clipShape(Circle())
        }
        .buttonStyle(.plain)
    }
}

#Preview {
    VStack(spacing: 16) {
        VButton(title: "Sign in with Apple", variant: .primary, isFullWidth: true) {}
        VButton(title: "Continue", variant: .secondary, isFullWidth: true) {}
        VButton(title: "Delete Account", variant: .destructive) {}
        VButton(title: "Cancel", variant: .ghost) {}
        VButton(title: "Learn more", variant: .link) {}
        VButton(title: "Loading...", isLoading: true) {}
        VButton(title: "Disabled", isDisabled: true) {}
    }
    .padding()
}

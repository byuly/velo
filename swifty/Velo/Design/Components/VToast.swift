import SwiftUI

struct VToast: View {
    let message: String
    var icon: String? = "checkmark.circle.fill"

    var body: some View {
        HStack(spacing: Tokens.Spacing.sm) {
            if let icon {
                Image(systemName: icon)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Tokens.Colors.primary)
            }
            Text(message)
                .font(Tokens.Typography.bodySmall)
                .fontWeight(.medium)
                .foregroundStyle(Tokens.Colors.foreground)
        }
        .padding(.horizontal, Tokens.Spacing.base)
        .padding(.vertical, Tokens.Spacing.md)
        .background(Tokens.Colors.background)
        .clipShape(Capsule())
        .shadow(
            color: Tokens.Shadow.lg.color,
            radius: Tokens.Shadow.lg.radius,
            x: Tokens.Shadow.lg.x,
            y: Tokens.Shadow.lg.y
        )
        .overlay(Capsule().stroke(Tokens.Colors.border, lineWidth: 1))
    }
}

#Preview {
    ZStack {
        Color.gray.opacity(0.1).ignoresSafeArea()
        VToast(message: "Profile updated")
    }
}

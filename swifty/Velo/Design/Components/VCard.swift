import SwiftUI

struct VCard<Content: View>: View {
    var padding: CGFloat = Tokens.Spacing.base
    var cornerRadius: CGFloat = Tokens.Radius.base
    @ViewBuilder let content: () -> Content

    var body: some View {
        content()
            .padding(padding)
            .background(Tokens.Colors.background)
            .clipShape(RoundedRectangle(cornerRadius: cornerRadius))
            .overlay(
                RoundedRectangle(cornerRadius: cornerRadius)
                    .stroke(Tokens.Colors.border, lineWidth: 1)
            )
            .cardShadow()
    }
}

#Preview {
    VCard {
        VStack(alignment: .leading, spacing: 8) {
            Text("Card Title").font(Tokens.Typography.h3)
            Text("Card content goes here").font(Tokens.Typography.bodyBase)
                .foregroundStyle(Tokens.Colors.mutedForeground)
        }
    }
    .padding()
}
